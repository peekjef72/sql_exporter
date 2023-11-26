//go:build !db2 && !mssql && !postgres && oracle

package main

var (
	exporter_namespace = "pg"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "Postgres Exporter"
	configEnvName         = "PG_CONFIG"
)
