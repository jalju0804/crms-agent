package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Interval != time.Minute {
		t.Fatalf("Interval = %v, want %v", cfg.Interval, time.Minute)
	}
	if cfg.OutputPath != "/var/lib/vm-metric-agent/samples.jsonl" {
		t.Fatalf("OutputPath = %q", cfg.OutputPath)
	}
	if cfg.RootPath != "/" {
		t.Fatalf("RootPath = %q", cfg.RootPath)
	}
	if len(cfg.NetworkExcludePrefixes) == 0 {
		t.Fatal("NetworkExcludePrefixes should have safe defaults")
	}
}

func TestLoadOverridesDefaultsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("interval: 10s\noutput_mode: both\noutput_path: /tmp/samples.jsonl\nroot_path: /mnt\nmetadata_url: http://metadata/openstack/latest/meta_data.json\ngnocchi:\n  endpoint: http://gnocchi:8041\n  username: admin\n  archive_policy_name: vm_guest_usage_default\nnetwork_exclude_prefixes:\n  - lo\n  - veth\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Interval != 10*time.Second {
		t.Fatalf("Interval = %v, want 10s", cfg.Interval)
	}
	if cfg.OutputPath != "/tmp/samples.jsonl" {
		t.Fatalf("OutputPath = %q", cfg.OutputPath)
	}
	if cfg.OutputMode != OutputModeBoth {
		t.Fatalf("OutputMode = %q", cfg.OutputMode)
	}
	if cfg.RootPath != "/mnt" {
		t.Fatalf("RootPath = %q", cfg.RootPath)
	}
	if cfg.Gnocchi.Endpoint != "http://gnocchi:8041" {
		t.Fatalf("Gnocchi.Endpoint = %q", cfg.Gnocchi.Endpoint)
	}
	if cfg.Gnocchi.ArchivePolicyName != "vm_guest_usage_default" {
		t.Fatalf("Gnocchi.ArchivePolicyName = %q", cfg.Gnocchi.ArchivePolicyName)
	}
	if cfg.MetadataURL != "http://metadata/openstack/latest/meta_data.json" {
		t.Fatalf("MetadataURL = %q", cfg.MetadataURL)
	}
	if got := cfg.NetworkExcludePrefixes; len(got) != 2 || got[0] != "lo" || got[1] != "veth" {
		t.Fatalf("NetworkExcludePrefixes = %#v", got)
	}
}

func TestLoadRejectsNonPositiveInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("interval: 0s\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for zero interval")
	}
}

func TestLoadRejectsGnocchiModeWithoutEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output_mode: gnocchi\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for gnocchi mode without endpoint")
	}
}

func TestLoadAcceptsGatewayModeWithEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("output_mode: gateway\ngateway:\n  endpoint: http://gateway:8080\n  token: secret\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.OutputMode != OutputModeGateway {
		t.Fatalf("OutputMode = %q", cfg.OutputMode)
	}
	if cfg.Gateway.Endpoint != "http://gateway:8080" {
		t.Fatalf("Gateway.Endpoint = %q", cfg.Gateway.Endpoint)
	}
	if cfg.Gateway.Token != "secret" {
		t.Fatalf("Gateway.Token = %q", cfg.Gateway.Token)
	}
}

func TestLoadRejectsGatewayModeWithoutEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output_mode: gateway\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for gateway mode without endpoint")
	}
}
