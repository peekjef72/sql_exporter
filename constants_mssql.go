//go:build !db2 && !hana && mssql && !oracle

package main

var (
	exporter_namespace = "mssql"
)

const (
	metricsPublishingPort = ":9399"
	exporter_name         = "mssql_exporter"
	configEnvName         = "MSSQL_CONFIG"
	driver_name           = "sqlserver"
)
