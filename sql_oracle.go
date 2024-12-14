//go:build !db2 && !hana && !mssql && oracle

package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/mattn/go-oci8"
	// register the Oracle OCI-8 driver
)

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//
// url format "oracle://<hostname>:<port>/<instance>?user%20id=<login>&password=<password>&database=<database>&protocol=...&options="
//
// or:
//
// INSTANCE=<instance>; DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;<br>
//
// ## parameters synonym => final value
// * server, hostname => server
// * uid, user, login => user id
// * pwd, passwd, password => password
//
//	 valid options are:
//		- loc
//	 - isolation
//	 - questionph
//	 - prefetch_rows
//	 - prefetch_memory
//	 - as
//	 - stmt_cache_size
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
		driver = "oci8"
	} else {
		driver = dsn[:idx]
	}

	// Adjust DSN, where necessary.
	var params map[string]string
	switch driver {
	case "oracle", "oci8":
		var err error
		if strings.HasPrefix(dsn, "oracle://") || strings.HasPrefix(dsn, "oci8://") {
			// "oracle://<hostname>:<port>/<database>?user%20id=<login>&password=<password>&database=<database>&protocol=..."
			params, err = splitConnectionStringURL(dsn)
			if err != nil {
				return "", err
			}
			// if strings.HasPrefix(dsn, "oracle://") {
			// }

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

			// server, sid := my_split(params["server"], "\\")
			// params["server"] = server
			// params["instance"] = sid
			// val, ok = params["instance"]
			// if !ok || val == "" {
			// 	return "", fmt.Errorf("database must be set")
			// }

			// 2 cases:
			// a) old format: only instance is specified
			// b) new for pdbs databases:
			//		instance and databases is specified
			val, ok = params["instance"]
			if !ok || val == "" {
				val, ok = params["database"]
				if !ok || val == "" {
					return "", fmt.Errorf("instance must be set")
				} else {
					// database is defined but not instance: switch values
					params["instance"] = params["database"]
					delete(params, "database")
				}
			}

			// oci8.ParseDSN
			// <user id>/<password>@(DESCRIPTION =
			//		(ADDRESS =
			//			(PROTOCOL = <protocol>)
			//			(host = <server>)
			//			(port = <port>)
			//  	)
			//		(CONNECT_DATA =
			//			(SID = <service name>)
			//		)
			// )
			new_dns := new(strings.Builder)
			// user
			new_dns.WriteString(url.QueryEscape(params["user id"]))
			new_dns.WriteString("/")
			// password
			new_dns.WriteString(url.QueryEscape(params["password"]))
			new_dns.WriteString("@")

			// DESCRIPTION
			new_dns.WriteString("(DESCRIPTION = ")

			//	ADDRESS
			new_dns.WriteString("(ADDRESS =")

			// Protocol
			new_dns.WriteString("(PROTOCOL = ")

			if params["protocol"] != "" {
				new_dns.WriteString(strings.ToUpper(params["protocol"]))
			} else {
				new_dns.WriteString("TCP")
			}
			new_dns.WriteString(") ")

			// Hostname
			new_dns.WriteString("(HOST =")
			new_dns.WriteString(params["server"])
			new_dns.WriteString(") ")

			// Port
			new_dns.WriteString(" ( PORT=")
			if params["port"] != "" {
				new_dns.WriteString(params["port"])
			} else {
				new_dns.WriteString("1531")
			}
			new_dns.WriteString(")")

			// END ADDRESS
			new_dns.WriteString(") ")

			// SID or SERVICE_NAME
			new_dns.WriteString("(CONNECT_DATA = (")
			if _, ok := params["database"]; ok {
				new_dns.WriteString(" SERVICE_NAME = ")
				new_dns.WriteString(params["database"])
			} else {
				new_dns.WriteString(" SID = ")
				new_dns.WriteString(params["instance"])
			}
			new_dns.WriteString("))")

			// END DESCRIPTION
			new_dns.WriteString(")")

			new_dns.WriteString("?")
			// others params
			var params_list = []string{
				"loc",
				"isolation",
				"questionph",
				"prefetch_rows",
				"prefetch_memory",
				"as",
				"stmt_cache_size",
			}
			for _, param := range params_list {
				val, err := params[param]
				if err {
					new_dns.WriteString(param)
					new_dns.WriteString("=")
					if params["protocol"] != "" {
						new_dns.WriteString(val)
						new_dns.WriteString("&")
					}
				}
			}

			dsn = new_dns.String()
			// add params to target symbol table
			symbol_table["params"] = params
		}

	default:
		return "", fmt.Errorf("driver '%s' not supported", driver)
	}

	// WARNING: display password in clear text in log !!!
	// logger.Debug(fmt.Sprintf("private dsn='%s", dsn))

	return dsn, nil
}

// Check if oracledb server returns an error message indicating
// that something is wrong with password or login so that cnx is reset
// and the next call, tries to recompute the login/passwd only if auth_key has changed.
//
// * "ORA-01005: null password given; logon denied\n"
//
// * "ORA-01017: invalid username/password; logon denied\n"
func check_login_error(err error) bool {
	check := false
	if strings.HasPrefix(err.Error(), "ORA-01005") ||
		strings.HasPrefix(err.Error(), "ORA-01017") {
		check = true
	}
	return check
}

// func my_split(s string, c string) (string, string) {
// 	i := strings.LastIndex(s, c)
// 	if i < 0 {
// 		return s, ""
// 	}
// 	return s[:i], s[i+len(c):]
// }
