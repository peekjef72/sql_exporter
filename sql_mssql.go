//go:build !db2 && !hana && mssql && !oracle

package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/microsoft/go-mssqldb" // register the MS-SQL driver
)

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//
// # MS SQL Server
//
// Using the https://github.com/denisenkom/go-mssqldb driver, DSN format (passed through to the driver unchanged):
//
// url format:
//
//	sqlserver://username:password@host:port/instance?param=value
//
// or DSN format:
//
//	DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
func BuildConnection(
	logger *slog.Logger,
	dsn string,
	auth AuthConfig,
	symbol_table map[string]interface{}) (string, error) {

	var driver string
	// Extract driver name from DSN.
	idx := strings.Index(dsn, "://")
	if idx == -1 {
		driver = "sqlserver"
	} else {
		driver = dsn[:idx]
	}

	// Adjust DSN, where necessary.
	var params map[string]string
	switch driver {
	case "sqlserver":
		var err error
		if strings.HasPrefix(dsn, "sqlserver://") {
			// "sqlserver://<hostname>:<port>/<path>?user%20id=<login>&password=<password>&database=<database>&protocol=..."
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

		val, ok := params["server"]
		if !ok || val == "" {
			return "", fmt.Errorf("server can't be empty")
		}

		val, ok = params["user id"]
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
		passwd := val
		if strings.HasPrefix(passwd, "/encrypted/") {
			if val, auth_key, err := BuildPasswd(logger, passwd, symbol_table); err == nil {
				params["password"] = val
				params["auth_key"] = auth_key
			}
			params["need_auth_key"] = "true"
		} else {
			params["password"] = val
			params["need_auth_key"] = "false"
		}

		// add params to target symbol table
		symbol_table["params"] = params

	default:
		return "", fmt.Errorf("driver '%s' not supported", driver)
	}

	// rebuild dsn from params because params may have changed
	return genDSNUrl(driver, params), nil
}

// generate DSN string in url format from parameters map
func genDSNUrl(driver string, params map[string]string) string {

	new_dns := new(strings.Builder)

	new_dns.WriteString(driver)
	new_dns.WriteString("://")

	// Hostname
	new_dns.WriteString(params["server"])

	// Port
	if params["port"] != "" {
		new_dns.WriteString(":")
		new_dns.WriteString(params["port"])
	}
	// instance
	if params["instance"] != "" {
		new_dns.WriteString("/")
		new_dns.WriteString(params["instance"])
	}

	param_idx := 0
	for key, val := range params {
		if key == "server" || key == "port" {
			continue
		}
		if param_idx == 0 {
			new_dns.WriteString("?")
		} else {
			new_dns.WriteString("&")
		}
		new_dns.WriteString(url.QueryEscape(key))
		new_dns.WriteString("=")
		new_dns.WriteString(url.QueryEscape(val))
		param_idx++
	}

	return new_dns.String()
}

// Check if mssql server returns an error message indicating
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
