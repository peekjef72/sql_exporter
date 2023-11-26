//go:build db2 && !oracle && !mssql && !postgres

package main

var (
	exporter_namespace = "db2"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "DB2 Exporter"
	configEnvName         = "DB2_CONFIG"
)
