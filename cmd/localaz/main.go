// Command localaz runs the localaz Azure emulator. A single process exposes
// each emulated Azure service on its own listener (a multi-port layout) so that
// users only ever run one container.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"localaz.dev/internal/servers/aadserver"
	"localaz.dev/internal/servers/armserver"
	"localaz.dev/internal/servers/blobserver"
	"localaz.dev/internal/servers/egserver"
	"localaz.dev/internal/servers/kvserver"
	"localaz.dev/internal/servers/monitorserver"
	"localaz.dev/internal/servers/queueserver"
	"localaz.dev/internal/servers/sbserver"
	"localaz.dev/internal/servers/tableserver"
	"localaz.dev/internal/servers/wpsserver"
	"localaz.dev/internal/stores/armstore"
	"localaz.dev/internal/stores/blobstore/fsstore"
	"localaz.dev/internal/stores/egstore"
	"localaz.dev/internal/stores/keyvaultstore"
	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/stores/sbstore"
	"localaz.dev/internal/stores/tablestore"
)

func main() {
	cfg := parseFlags()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	tlsCert, err := loadTLS(logger, cfg.dataDir, cfg.tlsCertFile, cfg.tlsKeyFile, cfg.advertiseHost)
	fatal(logger, "init tls", err)
	blobStore, err := fsstore.New(cfg.dataDir)
	fatal(logger, "init blob store", err)
	queueStore, err := queuestore.New(cfg.dataDir)
	fatal(logger, "init queue store", err)
	tableStore, err := tablestore.New(cfg.dataDir)
	fatal(logger, "init table store", err)
	aadServer, err := aadserver.New(cfg.dataDir)
	fatal(logger, "init aad server", err)
	keyVaultStore, err := keyvaultstore.New(cfg.dataDir)
	fatal(logger, "init key vault store", err)

	const scheme = "https"
	aadAuthority := controlURL(scheme, cfg.advertiseHost, cfg.aadAddr) + "/adfs"
	armStore := armstore.New(armstore.Config{
		CloudName:            cfg.cloudName,
		TenantID:             "adfs",
		LoginEndpoint:        controlURL(scheme, cfg.advertiseHost, cfg.aadAddr) + "/",
		ResourceManager:      controlURL(scheme, cfg.advertiseHost, cfg.armAddr),
		LogAnalyticsEndpoint: controlURL(scheme, cfg.advertiseHost, cfg.monitorAddr),
		StorageSuffix:        cfg.advertiseHost,
	})

	services := []service{
		{name: "blob", addr: cfg.blobAddr, handler: blobserver.New(blobStore)},
		{name: "queue", addr: cfg.queueAddr, handler: queueserver.New(queueStore)},
		{name: "table", addr: cfg.tableAddr, handler: tableserver.New(tableStore)},
		{name: "eventgrid", addr: cfg.eventGridAddr, handler: egserver.New(egstore.New())},
		{name: "webpubsub", addr: cfg.webPubSubAddr, handler: wpsserver.New()},
		{name: "monitor", addr: cfg.monitorAddr, handler: monitorserver.New(monitorstore.New())},
		{name: "aad", addr: cfg.aadAddr, handler: aadServer},
		{name: "arm", addr: cfg.armAddr, handler: armserver.New(armStore)},
		{name: "keyvault", addr: cfg.keyVaultAddr, handler: kvserver.New(keyVaultStore, aadAuthority)},
	}

	amqp := amqpService{addr: cfg.serviceBusAddr, server: sbserver.New(sbstore.New())}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, services, amqp, tlsCert); err != nil {
		os.Exit(1)
	}
}
