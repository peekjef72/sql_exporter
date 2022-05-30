//go:build !db2 && !hana && !mssql && oracle && !postgres

package main

var (
	exporter_namespace = "oracledb"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "oracledb_exporter"
	configEnvName         = "ORACLEDB_CONFIG"
)
