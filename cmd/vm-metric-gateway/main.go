package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crms-agent/internal/gateway"
	"crms-agent/internal/gatewayconfig"
	"crms-agent/internal/gnocchi"
	"crms-agent/internal/registry"
	"crms-agent/internal/sample"
)

func main() {
	configPath := flag.String("config", "/etc/vm-metric-gateway/config.yaml", "path to gateway config.yaml")
	flag.Parse()

	cfg, err := gatewayconfig.Load(*configPath)
	if err != nil {
		log.Fatalf("load gateway config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		log.Fatalf("run gateway: %v", err)
	}
}

func run(ctx context.Context, cfg gatewayconfig.Config) error {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	gnocchiClient := gnocchi.NewClient(cfg.Gnocchi.Endpoint, cfg.Gnocchi.Username, cfg.Gnocchi.Password, httpClient)

	ready := true
	if err := gnocchiClient.EnsureArchivePolicy(ctx, gnocchi.DefaultArchivePolicy(cfg.Gnocchi.ArchivePolicyName)); err != nil {
		ready = false
		log.Printf("ensure gnocchi archive policy: %v", err)
	}

	gatewayServer := gateway.NewServer(gateway.Options{
		AuthToken: cfg.AuthToken,
		Ready:     ready,
		Sink: gnocchiSink{
			client:            gnocchiClient,
			archivePolicyName: cfg.Gnocchi.ArchivePolicyName,
		},
		Registry: registry.New(),
	})
	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           gatewayServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("vm-metric-gateway listening on %s", cfg.ListenAddress)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

type gnocchiSink struct {
	client            gnocchi.Client
	archivePolicyName string
}

func (s gnocchiSink) SendSample(ctx context.Context, smpl sample.Sample) error {
	return s.client.SendSample(ctx, smpl, s.archivePolicyName)
}
