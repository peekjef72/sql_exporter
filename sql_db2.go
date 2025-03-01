//go:build db2 && !hana && !mssql && !oracle

package main

import (
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/ibmdb/go_ibm_db" // register the DB2 driver
)

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//
// # DB2 sql server
//
// Using the https://github.com/denisenkom/go-mssqldb driver, DSN format (passed through to the driver unchanged):
//
// url format:
// 		db2://<hostname>:<port>?user%20id=<login>&password=<password>&database=<database>&protocol=...
// DSN format!
// 		DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;

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
		//return nil, fmt.Errorf("missing driver in data source name. Expected format `<driver>://<dsn>`")
		driver = "db2"
	} else {
		driver = dsn[:idx]
	}

	// Adjust DSN, where necessary.
	var params map[string]string
	switch driver {
	case "db2":
		var err error
		if strings.HasPrefix(dsn, "db2://") {
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
					params["__auth_key"] = auth_key
				} else {
					return "", fmt.Errorf("unable to decrypt password")
				}
				params["__need_auth_key"] = "true"
			} else {
				params["password"] = val
				params["__need_auth_key"] = "false"
			}

			val, ok = params["database"]
			if !ok || val == "" {
				return "", fmt.Errorf("database must be set")
			}
			if params["port"] == "" {
				params["port"] = "60000"
			}

			if params["protocol"] == "" {
				params["protocol"] = "TCPIP"
			}

			// remove instance from url if any has been specified
			delete(params, "instance")

			// add params to target symbol table
			symbol_table["params"] = params
		}

	default:
		return "", fmt.Errorf("driver '%s' not supported", driver)
	}

	// rebuild dsn from params because params may have changed
	// dsn = genDSN(params)
	// // WARNING: display password in clear text in log !!!
	// logger.Debug(fmt.Sprintf("private dsn='%s", dsn))
	// return dsn, nil
	return genDSN(params), nil
}

// generate DSN string from parameter map
func genDSN(params map[string]string) string {

	new_dns := new(strings.Builder)
	// Hostname
	new_dns.WriteString("HOSTNAME=")
	new_dns.WriteString(params["server"])
	new_dns.WriteString("; ")

	// Port
	if params["port"] != "" {
		new_dns.WriteString("PORT=")
		new_dns.WriteString(params["port"])
		new_dns.WriteString("; ")
	}

	// Database
	new_dns.WriteString("DATABASE=")
	new_dns.WriteString(params["database"])
	new_dns.WriteString("; ")

	// Protocol
	if params["protocol"] != "" {
		new_dns.WriteString("PROTOCOL=")
		new_dns.WriteString(params["protocol"])
		new_dns.WriteString("; ")
	}

	// user
	new_dns.WriteString("UID=")
	new_dns.WriteString(params["user id"])
	new_dns.WriteString("; ")

	// password
	new_dns.WriteString("PWD=")
	new_dns.WriteString(params["password"])
	new_dns.WriteString("; ")

	return new_dns.String()
}

// Check if db2 server returns an error message indicating
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
