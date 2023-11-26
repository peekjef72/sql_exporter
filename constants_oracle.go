//go:build !db2 && !mssql && !postgres && oracle

package main

var (
	exporter_namespace = "oracledb"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "Oracle Exporter"
	configEnvName         = "ORACLE_CONFIG"
)
