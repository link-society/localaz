package main

import (
	"flag"
	"os"
)

// config holds the resolved command-line / environment options for localaz.
type config struct {
	blobAddr       string
	queueAddr      string
	tableAddr      string
	eventGridAddr  string
	webPubSubAddr  string
	monitorAddr    string
	aadAddr        string
	armAddr        string
	keyVaultAddr   string
	serviceBusAddr string
	managementAddr string
	dataDir        string
	cloudName      string
	advertiseHost  string
	tlsCertFile    string
	tlsKeyFile     string
	healthcheck    bool
}

// parseFlags resolves configuration from command-line flags, falling back to
// environment variables and then built-in defaults.
func parseFlags() config {
	var c config
	flag.StringVar(&c.blobAddr, "blob-addr", envOr("LOCALAZ_BLOB_ADDR", ":10000"), "blob service listen address")
	flag.StringVar(&c.queueAddr, "queue-addr", envOr("LOCALAZ_QUEUE_ADDR", ":10001"), "queue service listen address")
	flag.StringVar(&c.tableAddr, "table-addr", envOr("LOCALAZ_TABLE_ADDR", ":10002"), "table service listen address")
	flag.StringVar(&c.eventGridAddr, "eventgrid-addr", envOr("LOCALAZ_EVENTGRID_ADDR", ":10003"), "event grid service listen address")
	flag.StringVar(&c.webPubSubAddr, "webpubsub-addr", envOr("LOCALAZ_WEBPUBSUB_ADDR", ":10004"), "web pubsub service listen address")
	flag.StringVar(&c.monitorAddr, "monitor-addr", envOr("LOCALAZ_MONITOR_ADDR", ":10005"), "monitor logs service listen address")
	flag.StringVar(&c.aadAddr, "aad-addr", envOr("LOCALAZ_AAD_ADDR", ":10006"), "entra id (aad) service listen address")
	flag.StringVar(&c.armAddr, "arm-addr", envOr("LOCALAZ_ARM_ADDR", ":10007"), "resource manager (arm) service listen address")
	flag.StringVar(&c.keyVaultAddr, "keyvault-addr", envOr("LOCALAZ_KEYVAULT_ADDR", ":10008"), "key vault service listen address")
	flag.StringVar(&c.serviceBusAddr, "servicebus-addr", envOr("LOCALAZ_SERVICEBUS_ADDR", ":5672"), "service bus AMQP listen address")
	flag.StringVar(&c.managementAddr, "management-addr", envOr("LOCALAZ_MANAGEMENT_ADDR", ":8000"), "management service listen address: health probe, certificates, and future endpoints (plain HTTP, no TLS)")
	flag.StringVar(&c.dataDir, "data", envOr("LOCALAZ_DATA_DIR", "/data"), "directory for persisted state")
	flag.StringVar(&c.cloudName, "arm-cloud-name", envOr("LOCALAZ_ARM_CLOUD_NAME", "localaz"), "cloud name advertised by the ARM metadata document")
	flag.StringVar(&c.advertiseHost, "advertise-host", envOr("LOCALAZ_ADVERTISE_HOST", "127.0.0.1"), "host clients use to reach the control-plane services")
	flag.StringVar(&c.tlsCertFile, "tls-cert", envOr("LOCALAZ_TLS_CERT", ""), "PEM certificate to serve TLS with (a self-signed one is generated when unset)")
	flag.StringVar(&c.tlsKeyFile, "tls-key", envOr("LOCALAZ_TLS_KEY", ""), "PEM private key matching -tls-cert")
	flag.BoolVar(&c.healthcheck, "healthcheck", false, "probe the management server's health endpoint and exit (0 ready, 1 otherwise); used by the container HEALTHCHECK")
	flag.Parse()
	return c
}

// envOr returns the value of the named environment variable, or fallback when
// it is unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
