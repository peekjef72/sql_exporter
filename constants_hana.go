//go:build !db2 && hana && !mssql && !oracle

package main

var (
	exporter_namespace = "hana"
)

const (
	metricsPublishingPort = ":9658"
	exporter_name         = "hana_exporter"
	configEnvName         = "HANA_CONFIG"
	driver_name           = "hdb"
)
