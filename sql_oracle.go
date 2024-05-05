//go:build !db2 && !hana && !mssql && oracle

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	_ "github.com/mattn/go-oci8"
	"github.com/peekjef72/httpapi_exporter/encrypt"
	// register the Oracle OCI-8 driver
)

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//

func OpenConnection(
	ctx context.Context,
	logContext []interface{},
	logger log.Logger,
	dsn string,
	auth AuthConfig,
	maxConns, maxIdleConns int,
	symbol_table map[string]interface{}) (*sql.DB, error) {
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
				return nil, err
			}
			// if strings.HasPrefix(dsn, "oracle://") {
			// }

		} else {
			// DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
			params, err = splitRawConnectionStringDSN(dsn)
			if err != nil {
				return nil, err
			}

		}

		val, ok := params["server"]
		if !ok || val == "" {
			return nil, fmt.Errorf("server can't be empty")
		}

		if auth.Username != "" {
			params["user id"] = auth.Username
		} else {
			val, ok = params["user id"]
			if !ok || val == "" {
				return nil, fmt.Errorf("user Id can't be empty")
			}
		}

		if auth.Password != "" {
			passwd := string(auth.Password)
			if strings.HasPrefix(passwd, "/encrypted/") {
				ciphertext := passwd[len("/encrypted/"):]
				level.Debug(logger).Log(
					"module", "sql::OpenConnection()",
					"ciphertext", ciphertext)
				auth_key := GetMapValueString(symbol_table, "auth_key")
				level.Debug(logger).Log(
					"module", "sql::OpenConnection()",
					"auth_key", auth_key)
				if auth_key == "" {
					return nil, fmt.Errorf("password is encrypt and not ciphertext provided (auth_key)")
				}
				cipher, err := encrypt.NewAESCipher(auth_key)
				if err != nil {
					err := fmt.Errorf("can't obtain cipher to decrypt")
					// level.Error(c.logger).Log("errmsg", err)
					return nil, err
				}
				passwd, err = cipher.Decrypt(ciphertext, true)
				if err != nil {
					err := fmt.Errorf("invalid key provided to decrypt")
					// level.Error(c.logger).Log("errmsg", err)
					return nil, err
				}
				params["password"] = passwd
			}

		} else {
			_, ok = params["password"]
			if !ok {
				return nil, fmt.Errorf("password has to be set")
			}
		}

		val, ok = params["instance"]
		if !ok || val == "" {
			return nil, fmt.Errorf("database must be set")
		} else {
			params["service name"] = params["instance"]
			delete(params, "instance")
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

		// SID
		new_dns.WriteString("(CONNECT_DATA = (SID = ")
		new_dns.WriteString(params["service name"])
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

		driver = "oci8"
	default:
		return nil, fmt.Errorf("driver '%s' not supported", driver)
	}

	// Open the DB handle in a separate goroutine so we can terminate early if the context closes.
	var (
		conn *sql.DB
		err  error
		ch   = make(chan error)
	)
	go func() {
		conn, err = sql.Open(driver, dsn)
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
		if err != nil {
			return nil, err
		}
	}

	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetMaxOpenConns(maxConns)

	logContext = append(logContext, "msg", fmt.Sprintf("Database handle successfully opened with driver %s.", driver))
	level.Debug(logger).Log(logContext...)

	return conn, nil
}

func my_split(s string, c string) (string, string) {
	i := strings.LastIndex(s, c)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+len(c):]
}
