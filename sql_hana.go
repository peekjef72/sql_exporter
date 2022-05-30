//go:build !db2 && hana && !mssql && !oracle && !postgres

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/SAP/go-hdb/driver" // register the sap hana driver

	"github.com/peekjef72/passwd_encrypt/encrypt"
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
func OpenConnection(
	ctx context.Context,
	logContext []interface{},
	logger *slog.Logger,
	dsn string,
	auth AuthConfig,
	maxConns, maxIdleConns int,
	symbol_table map[string]interface{}) (*sql.DB, error) {

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
			logger.Debug("debug ciphertext",
				"ciphertext", ciphertext)
			auth_key := GetMapValueString(symbol_table, "auth_key")
			logger.Debug(
				"debug authkey",
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

		// add params to target symbol table
		symbol_table["params"] = params

	default:
		return nil, fmt.Errorf("driver '%s' not supported", driver)
	}

	// rebuild dsn from params because params may have changed
	dsn = GenDSNUrlHana(driver, params)

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
	logger.Debug("msg_stack",
		logContext...)

	return conn, nil
}

// generate DSN string in url format from parameters map
func GenDSNUrlHana(driver string, params map[string]string) string {

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
