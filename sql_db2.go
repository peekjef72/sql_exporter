//go:build db2 && !hana && !mssql && !oracle && !postgres

package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	_ "github.com/ibmdb/go_ibm_db" // register the DB2 driver
	"github.com/peekjef72/passwd_encrypt/encrypt"
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
				return nil, err
			}
		} else {
			// DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
			params, err = splitRawConnectionStringDSN(dsn)
			if err != nil {
				return nil, err
			}
		}

		val, ok := params["user id"]
		if !ok || val == "" {
			if auth.Username != "" {
				params["user id"] = auth.Username
			} else {
				return nil, fmt.Errorf("user Id can't be empty")
			}
		}

		val, ok = params["password"]
		if !ok || val == "" {
			if auth.Password != "" {
				val = string(auth.Password)
			} else {
				return nil, fmt.Errorf("password has to be set")
			}
		}
		passwd := val
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

		val, ok = params["database"]
		if !ok || val == "" {
			return nil, fmt.Errorf("database must be set")
		}
		if params["port"] == "" {
			params["port"] = "60000"
		}

		if params["protocol"] == "" {
			params["protocol"] = "TCP"
		}

		// remove instance from url if any has been specified
		delete(params, "instance")

		// add params to target symbol table
		symbol_table["params"] = params

		driver = "go_ibm_db"
	default:
		return nil, fmt.Errorf("driver '%s' not supported", driver)
	}

	// rebuild dsn from params because params may have changed
	dsn = GenDSN(params)

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
