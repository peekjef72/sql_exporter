//go:build oracle

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	go_ora "github.com/sijms/go-ora/v2"
)

func OpenConnection(
	ctx context.Context,
	logContext []interface{},
	logger *slog.Logger,
	driver string,
	dsn string,
	maxConns, maxIdleConns int,
) (*sql.DB, error) {

	// Open the DB handle in a separate goroutine so we can terminate early if the context closes.
	var (
		conn *sql.DB
		ch   = make(chan error)
	)
	connector := go_ora.NewConnector(dsn)

	go func() {
		conn = sql.OpenDB(connector)
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
		if conn == nil {
			return nil, fmt.Errorf("connection is invalid")
		}
	}

	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetMaxOpenConns(maxConns)

	logContext = append(logContext, "msg", fmt.Sprintf("Database handle successfully opened with driver %s.", driver))
	logger.Debug("msg_stack",
		logContext...)
	return conn, nil
}
