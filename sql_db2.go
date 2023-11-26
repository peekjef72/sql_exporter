//go:build db2 && !mssql && !oracle && !postgres

package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/log/level"
	_ "github.com/ibmdb/go_ibm_db" // register the DB2 driver
	"github.com/peekjef72/sql_exporter/encrypt"
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
// # MySQL
//
// Using the https://github.com/go-sql-driver/mysql driver, DSN format (passed to the driver stripped of the `mysql://`
// prefix):
//
//	mysql://username:password@protocol(host:port)/dbname?param=value
//
// # PostgreSQL
//
// Using the https://godoc.org/github.com/lib/pq driver, DSN format (passed through to the driver unchanged):
//
//	postgres://username:password@host:port/dbname?param=value
//
// # MS SQL Server
//
// Using the https://github.com/denisenkom/go-mssqldb driver, DSN format (passed through to the driver unchanged):
//
//	sqlserver://username:password@host:port/instance?param=value
//
// # Clickhouse
//
// Using the https://github.com/kshvakov/clickhouse driver, DSN format (passed to the driver with the`clickhouse://`
// prefix replaced with `tcp://`):
//
//	clickhouse://host:port?username=username&password=password&database=dbname&param=value
func OpenConnection(
	ctx context.Context,
	t *target) (*sql.DB, error) {
	// logContext []interface{},
	// logger log.Logger,
	// dsn string,
	// maxConns, maxIdleConns int,
	// symbol_table map[string]interface{}) (*sql.DB, error) {
	var (
		driver string
		err    error
	)

	// Extract driver name from DSN.
	dsn := string(t.config.DSN)
	idx := strings.Index(dsn, "://")
	if idx == -1 {
		//return nil, fmt.Errorf("missing driver in data source name. Expected format `<driver>://<dsn>`")
		driver = "db2"
	} else {
		driver = dsn[:idx]
	}

	// Adjust DSN, where necessary.
	var params map[string]string
	if driver != "db2" {
		return nil, fmt.Errorf("driver '%s' not supported", driver)
	}
	if strings.HasPrefix(dsn, "db2://") {
		// "db2://<hostname>:<port>?user%20id=<login>&password=<password>&database=<database>&protocol=..."
		params, err = splitConnectionStringURL(dsn)
		if err != nil {
			return nil, err
		}
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
	val, ok = params["user id"]
	if !ok || val == "" {
		return nil, fmt.Errorf("user Id can't be empty")
	}
	_, ok = params["password"]
	if !ok {
		return nil, fmt.Errorf("password has to be set")
	}
	val, ok = params["database"]
	if !ok || val == "" {
		return nil, fmt.Errorf("database must be set")
	}

	auth_set, _ := GetMapValueBool(symtab, "auth_set")
	if !t.config.AuthName {
		if auth_mode == "basic" {
			passwd := GetMapValueString(symtab, "password")
			if params.Password != "" {
				old_values["password"] = passwd
				passwd = params.Password
				symtab["password"] = passwd
			}
			if strings.Contains(passwd, "/encrypted/") {
				ciphertext := passwd[len("/encrypted/"):]
				level.Debug(c.logger).Log(
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger),
					"ciphertext", ciphertext)

				user := GetMapValueString(symtab, "user")
				if params.Username != "" {
					old_values["user"] = user
					user = params.Username
					symtab["user"] = user
				}
				auth_key := GetMapValueString(symtab, "auth_key")
				level.Debug(c.logger).Log(
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger),
					"auth_key", auth_key)
				cipher, err := encrypt.NewAESCipher(auth_key)
				if err != nil {
					err := fmt.Errorf("can't obtain cipher to decrypt")
					// level.Error(c.logger).Log("errmsg", err)
					return err
				}
				passwd, err = cipher.Decrypt(ciphertext, true)
				if err != nil {
					err := fmt.Errorf("invalid key provided to decrypt")
					// level.Error(c.logger).Log("errmsg", err)
					return err
				}
				c.client.SetBasicAuth(user, passwd)
				passwd = ""
				symtab["auth_set"] = true
				delete(symtab, "auth_key")
			}
		} else if auth_mode == "token" {
			auth_token := GetMapValueString(symtab, "auth_token")
			if params.Token != "" {
				old_values["auth_token"] = auth_token
				auth_token = params.Token
				symtab["auth_token"] = auth_token
			}
			if auth_token != "" {
				c.client.SetAuthToken(auth_token)
			}
		}
	}

	new_dns := new(strings.Builder)
	// Hostname
	new_dns.WriteString("HOSTNAME=")
	new_dns.WriteString(params["server"])
	new_dns.WriteString("; ")

	// Port
	new_dns.WriteString("PORT=")
	if params["port"] != "" {
		new_dns.WriteString(params["port"])
		new_dns.WriteString("; ")
	} else {
		new_dns.WriteString("60000; ")
	}
	// Database
	new_dns.WriteString("DATABASE=")
	new_dns.WriteString(params["database"])
	new_dns.WriteString("; ")

	// Protocol
	new_dns.WriteString("PROTOCOL=")
	if params["protocol"] != "" {
		new_dns.WriteString(params["protocol"])
		new_dns.WriteString("; ")
	} else {
		new_dns.WriteString("TCPIP; ")
	}
	// user
	new_dns.WriteString("UID=")
	new_dns.WriteString(params["user id"])
	new_dns.WriteString("; ")
	// password
	new_dns.WriteString("PWD=")
	new_dns.WriteString(params["password"])
	new_dns.WriteString("; ")

	dsn = new_dns.String()
	// add params to target symbol table
	symbol_table["params"] = params

	driver = "go_ibm_db"

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

	conn.SetMaxIdleConns(t.globalConfig.MaxIdleConns)
	conn.SetMaxOpenConns(t.globalConfig.MaxConns)

	t.logContext = append(t.logContext, "msg", fmt.Sprintf("Database handle successfully opened with driver %s.", driver))
	level.Debug(t.logger).Log(t.logContext...)

	return conn, nil
}
