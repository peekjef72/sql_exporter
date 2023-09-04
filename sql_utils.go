package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"

	_ "github.com/denisenkom/go-mssqldb" // register the MS-SQL driver
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

	if len(u.Path) > 0 {
		res["server"] = host + "\\" + u.Path[1:]
	} else {
		res["server"] = host
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

//
//#DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;
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
