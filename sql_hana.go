//go:build !db2 && hana && !mssql && !oracle

package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/SAP/go-hdb/driver" // register the sap hana driver
)

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//
// Below is the list of supported databases (with built in drivers) and their DSN formats. Unfortunately there is no
// dynamic way of loading a third party driver library (as e.g. with Java classpaths), so any driver additions require
// a binary rebuild.
//
// ## DSN format:
//
// hdb://<user>:<password>@<host>:<port>/<instance>?parameters
// ##Valid parameters are:
//
// * databaseName=<dbname>
// * defaultSchema=<schema>
// * timeout=<timeout_seconds>
// * pingInterval=<intervanl_seconds>
// * TLSRootCAFile=<>
// * TLSServerName=<>
// * TLSInsecureSkipVerify=<>
func BuildConnection(
	logger *slog.Logger,
	dsn string,
	auth AuthConfig,
	symbol_table map[string]any,
	check_only bool) (string, error) {

	var driver string
	// Extract driver name from DSN.
	idx := strings.Index(dsn, "://")
	if idx == -1 {
		driver = "hdb"
	} else {
		driver = dsn[:idx]
	}

	// Adjust DSN, where necessary.
	var params map[string]string
	switch driver {
	case "hdb":
		var err error
		if strings.HasPrefix(dsn, "hdb://") {
			// "db2://<hostname>:<port>?user%20id=<login>&password=<password>&database=<database>&protocol=..."
			params, err = splitConnectionStringURL(dsn)
			if err != nil {
				return "", err
			}
		} else {
			// DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
			params, err = splitRawConnectionStringDSN(dsn)
			if err != nil {
				return "", err
			}
		}

		val, ok := params["user id"]
		if !ok || val == "" {
			if auth.Username != "" {
				params["user id"] = auth.Username
			} else {
				return "", fmt.Errorf("user Id can't be empty")
			}
		}

		val, ok = params["password"]
		if !ok || val == "" {
			if auth.Password != "" {
				val = string(auth.Password)
			} else {
				return "", fmt.Errorf("password has to be set")
			}
		}

		if !check_only && symbol_table != nil {
			passwd := val
			if strings.HasPrefix(passwd, "/encrypted/") {
				if val, auth_key, err := BuildPasswd(logger, passwd, symbol_table); err == nil {
					params["password"] = val
					params["auth_key"] = auth_key
				} else {
					return "", fmt.Errorf("unable to decrypt password")
				}
				params["need_auth_key"] = "true"
			} else {
				params["password"] = val
				params["need_auth_key"] = "false"
			}

			// add params to target symbol table
			symbol_table["params"] = params
		}

	default:
		return "", fmt.Errorf("driver '%s' not supported", driver)
	}

	// rebuild dsn from params because params may have changed
	return genDSNUrlHana(driver, params), nil
}

// generate DSN string in url format from parameters map
func genDSNUrlHana(driver string, params map[string]string) string {

	new_dns := new(strings.Builder)

	new_dns.WriteString(driver)
	new_dns.WriteString("://")

	// username
	new_dns.WriteString(url.QueryEscape(params["user id"]))
	new_dns.WriteString(":")

	// username
	new_dns.WriteString(url.QueryEscape(params["password"]))
	new_dns.WriteString("@")

	// Hostname
	new_dns.WriteString(params["server"])

	// Port
	if params["port"] != "" {
		new_dns.WriteString(":")
		new_dns.WriteString(params["port"])
	}
	// instance
	if params["instance"] != "" {
		params["databaseName"] = params["instance"]
		delete(params, "instance")
	}

	var valid_params = [...]string{
		"databaseName",
		"defaultSchema",
		"timeout",
		"pingInterval",
		"TLSRootCAFile",
		"TLSServerName",
		"TLSInsecureSkipVerify",
	}

	for idx, key := range valid_params {
		if val, ok := params[key]; ok {
			if idx == 0 {
				new_dns.WriteString("?")
			} else {
				new_dns.WriteString("&")
			}
			new_dns.WriteString(url.QueryEscape(key))
			new_dns.WriteString("=")
			new_dns.WriteString(url.QueryEscape(val))
		}
	}

	return new_dns.String()
}

// Check if hanasql server returns an error message indicating
// that something is wrong with password or login so that cnx is reset
// and the next call, tries to recompute the login/passwd only if auth_key has changed.
//
// "ORA-01005: null password given; logon denied\n"
//
// "ORA-01017: invalid username/password; logon denied\n
func check_login_error(err error) bool {
	check := false
	if strings.HasPrefix(err.Error(), "ORA-01005") ||
		strings.HasPrefix(err.Error(), "ORA-01017") {
		check = true
	}
	return check
}
