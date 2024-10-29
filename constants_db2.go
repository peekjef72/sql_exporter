//go:build db2 && !hana && !mssql && !oracle

package main

var (
	exporter_namespace = "db2"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "db2_exporter"
	configEnvName         = "DB2_CONFIG"
	driver_name           = "go_ibm_db"
)
