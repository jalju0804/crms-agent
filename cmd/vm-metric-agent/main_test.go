package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"crms-agent/internal/collector"
	"crms-agent/internal/config"
	"crms-agent/internal/gatewayclient"
	"crms-agent/internal/gnocchi"
	"crms-agent/internal/metadata"
	"crms-agent/internal/sample"
)

func TestCollectAndWriteSendsGatewayModeToGatewayOnly(t *testing.T) {
	var got sample.Sample
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/samples" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "samples.jsonl")
	cfg := config.Config{
		OutputMode: config.OutputModeGateway,
		OutputPath: outputPath,
		Gateway: config.GatewayConfig{
			Endpoint: server.URL,
			Token:    "secret",
		},
	}
	prev := collector.Snapshot{CPU: collector.CPUStat{Total: 100, Idle: 50}}
	next := collector.Snapshot{
		CPU:        collector.CPUStat{Total: 200, Idle: 100},
		Memory:     collector.MemoryMetrics{UsedPercent: 25, AvailableBytes: 1024},
		Filesystem: collector.FilesystemMetrics{UsedPercent: 50},
		Disk:       collector.DiskMetrics{ReadBytes: 10, WriteBytes: 20},
		Network:    collector.NetworkMetrics{RXBytes: 30, TXBytes: 40},
		Agent:      collector.AgentMetrics{RSSBytes: 2048},
	}

	err := collectAndWrite(
		t.Context(),
		cfg,
		sample.NewWriter(outputPath),
		gnocchi.Client{},
		gatewayclient.New(server.URL, "secret", server.Client()),
		metadata.Info{InstanceID: "vm-1", ProjectID: "project-1", Hostname: "host-1"},
		nil,
		prev,
		next,
		time.Second,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.InstanceID != "vm-1" {
		t.Fatalf("InstanceID = %q", got.InstanceID)
	}
	if got.Metrics["guest.cpu.used_percent"] != 50 {
		t.Fatalf("guest.cpu.used_percent = %v", got.Metrics["guest.cpu.used_percent"])
	}
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gateway mode should not write JSONL, stat err = %v", err)
	}
}
