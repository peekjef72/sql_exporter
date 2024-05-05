package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// PingDB is a wrapper around sql.DB.PingContext() that terminates as soon as the context is closed.
//
// sql.DB does not actually pass along the context to the driver when opening a connection (which always happens if the
// database is down) and the driver uses an arbitrary timeout which may well be longer than ours. So we run the ping
// call in a goroutine and terminate immediately if the context is closed.
func PingDB(ctx context.Context, conn *sql.DB) error {
	ch := make(chan error, 1)

	go func() {
		ch <- conn.PingContext(ctx)
		close(ch)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

func splitConnectionStringURL(dsn string) (map[string]string, error) {
	res := map[string]string{}

	u, err := url.Parse(dsn)
	if err != nil {
		return res, err
	}

	if u.User != nil {
		res["user id"] = u.User.Username()
		p, exists := u.User.Password()
		if exists {
			// if strings.HasPrefix(p, "{") && strings.HasSuffix(p,"}") {
			// 	p = p[1:len(p)-1]
			// }
			res["password"] = p
		}
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	res["server"] = host
	if len(u.Path) > 0 {
		res["instance"] = u.Path[1:]
	}

	if len(port) > 0 {
		res["port"] = port
	}

	query := u.Query()
	for k, v := range query {
		if len(v) > 1 {
			return res, fmt.Errorf("key %s provided more than once", k)
		}
		res[strings.ToLower(k)] = v[0]
	}

	return res, nil
}

// #DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
func splitRawConnectionStringDSN(dsn string) (map[string]string, error) {
	res := map[string]string{}
	m := make(url.Values)

	for dsn != "" {
		var key string
		key, dsn, _ = strings.Cut(dsn, ";")
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")
		key = strings.Trim(key, " ")
		key = strings.ToLower(key)

		value = strings.Trim(value, " ")
		m[key] = append(m[key], value)
	}

	for k, v := range m {
		if len(v) > 1 {
			return res, fmt.Errorf("key %s provided more than once", k)
		}
		switch k {
		case "hostname":
			k = "server"
		case "uid", "user", "login":
			k = "user id"
		case "pwd", "passwd":
			k = "password"
		}
		res[k] = v[0]
	}

	return res, nil
}

// extract key from symbol table returning a value cast to desired type
func GetMapValueString(symtab map[string]any, key string) string {
	var value string
	if value_raw, ok := symtab[key]; ok {
		switch value_val := value_raw.(type) {
		case string:
			value = value_val
		case int:
			value = fmt.Sprintf("%d", value_val)
		default:
			value = ""
		}
	}
	return value
}

// generate DSN string from parameter map
func GenDSN(params map[string]string) string {

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

// generate DSN string in url format from parameters map
func GenDSNUrl(driver string, params map[string]string) string {

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
