package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crms-agent/internal/collector"
	"crms-agent/internal/config"
	"crms-agent/internal/gatewayclient"
	"crms-agent/internal/gnocchi"
	"crms-agent/internal/metadata"
	"crms-agent/internal/sample"
)

func main() {
	configPath := flag.String("config", "/etc/vm-metric-agent/config.yaml", "path to config.yaml")
	once := flag.Bool("once", false, "collect one sample and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, *once); err != nil {
		log.Fatalf("run agent: %v", err)
	}
}

func run(ctx context.Context, cfg config.Config, once bool) error {
	writer := sample.NewWriter(cfg.OutputPath)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	gnocchiHTTPClient := &http.Client{Timeout: 15 * time.Second}
	gnocchiClient := gnocchi.NewClient(cfg.Gnocchi.Endpoint, cfg.Gnocchi.Username, cfg.Gnocchi.Password, gnocchiHTTPClient)
	gatewayHTTPClient := &http.Client{Timeout: 15 * time.Second}
	gatewayClient := gatewayclient.New(cfg.Gateway.Endpoint, cfg.Gateway.Token, gatewayHTTPClient)
	var metadataInfo metadata.Info
	var metadataErr error
	var errorCount uint64

	if cfg.OutputMode == config.OutputModeGnocchi || cfg.OutputMode == config.OutputModeBoth {
		if err := gnocchiClient.EnsureArchivePolicy(ctx, gnocchi.DefaultArchivePolicy(cfg.Gnocchi.ArchivePolicyName)); err != nil {
			errorCount++
			log.Printf("ensure gnocchi archive policy: %v", err)
		}
	}

	refreshMetadata := func() {
		info, err := metadata.Fetch(ctx, httpClient, cfg.MetadataURL)
		if err != nil {
			metadataErr = err
			errorCount++
			return
		}
		metadataInfo = info
		metadataErr = nil
	}
	refreshMetadata()

	prev, err := collector.Collect(cfg.RootPath, cfg.NetworkExcludePrefixes, cfg.DiskExcludePrefixes)
	if err != nil {
		errorCount++
		return err
	}

	if once {
		return collectAndWrite(ctx, cfg, writer, gnocchiClient, gatewayClient, metadataInfo, metadataErr, prev, prev, 0, errorCount)
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			start := time.Now()
			next, err := collector.Collect(cfg.RootPath, cfg.NetworkExcludePrefixes, cfg.DiskExcludePrefixes)
			if err != nil {
				errorCount++
				log.Printf("collect metrics: %v", err)
				continue
			}
			if metadataInfo.InstanceID == "" {
				refreshMetadata()
			}
			err = collectAndWrite(ctx, cfg, writer, gnocchiClient, gatewayClient, metadataInfo, metadataErr, prev, next, time.Since(start), errorCount)
			if err != nil {
				errorCount++
				log.Printf("write sample: %v", err)
				continue
			}
			prev = next
		}
	}
}

func collectAndWrite(ctx context.Context, cfg config.Config, writer sample.Writer, gnocchiClient gnocchi.Client, gatewayClient gatewayclient.Client, info metadata.Info, metadataErr error, prev, next collector.Snapshot, loopDuration time.Duration, errorCount uint64) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	s := sample.Sample{
		Timestamp:        time.Now().UTC(),
		InstanceID:       info.InstanceID,
		ProjectID:        info.ProjectID,
		Hostname:         info.Hostname,
		Name:             info.Name,
		AvailabilityZone: info.AvailabilityZone,
		Metrics: map[string]float64{
			"guest.cpu.used_percent":        collector.CPUUsagePercent(prev.CPU, next.CPU),
			"guest.memory.used_percent":     next.Memory.UsedPercent,
			"guest.memory.available_bytes":  float64(next.Memory.AvailableBytes),
			"guest.filesystem.used_percent": next.Filesystem.UsedPercent,
			"guest.disk.read_bytes":         float64(next.Disk.ReadBytes),
			"guest.disk.write_bytes":        float64(next.Disk.WriteBytes),
			"guest.net.rx_bytes":            float64(next.Network.RXBytes),
			"guest.net.tx_bytes":            float64(next.Network.TXBytes),
			"agent.loop_duration_ms":        float64(loopDuration.Milliseconds()),
			"agent.error_count":             float64(errorCount),
			"agent.rss_bytes":               float64(next.Agent.RSSBytes),
		},
	}
	if metadataErr != nil {
		s.Errors = append(s.Errors, fmt.Sprintf("metadata: %v", metadataErr))
	}
	size := estimatedJSONLSize(s)
	s.Metrics["agent.sample_size_bytes"] = float64(size)
	if cfg.OutputMode == config.OutputModeJSONL || cfg.OutputMode == config.OutputModeBoth {
		if _, err := writer.Append(s); err != nil {
			return err
		}
	}
	if cfg.OutputMode == config.OutputModeGnocchi || cfg.OutputMode == config.OutputModeBoth {
		if err := gnocchiClient.SendSample(ctx, s, cfg.Gnocchi.ArchivePolicyName); err != nil {
			return err
		}
	}
	if cfg.OutputMode == config.OutputModeGateway {
		if err := gatewayClient.SendSample(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func estimatedJSONLSize(s sample.Sample) int {
	body, err := json.Marshal(s)
	if err != nil {
		return 0
	}
	return len(body) + 1
}
