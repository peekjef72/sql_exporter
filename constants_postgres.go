//go:build !db2 && !hana && !mssql && !oracle && postgres

package main

var (
	exporter_namespace = "pg"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "postgres_exporter"
	configEnvName         = "PG_CONFIG"
)
