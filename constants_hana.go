//go:build !db2 && hana && !mssql && !oracle && !postgres

package main

var (
	exporter_namespace = "hana"
)

const (
	metricsPublishingPort = ":9658"
	exporter_name         = "hana_exporter"
	configEnvName         = "HANA_CONFIG"
)
