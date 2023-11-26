//go:build !db2 && !oracle && !postgres && mssql

package main

var (
	exporter_namespace = "mssql"
)

const (
	metricsPublishingPort   = ":9399"
	exporter_name           = "mssql_exporter"
	exporter_name_for_human = "MSSQL Exporter"
	configEnvName           = "MSSQL_CONFIG"
)
