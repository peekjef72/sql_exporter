//go:build !oracle

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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
