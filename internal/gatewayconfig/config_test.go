package gatewayconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddress != ":8080" {
		t.Fatalf("ListenAddress = %q", cfg.ListenAddress)
	}
	if cfg.Gnocchi.Username != "admin" {
		t.Fatalf("Gnocchi.Username = %q", cfg.Gnocchi.Username)
	}
	if cfg.Gnocchi.ArchivePolicyName != "vm_guest_usage_default" {
		t.Fatalf("ArchivePolicyName = %q", cfg.Gnocchi.ArchivePolicyName)
	}
}

func TestLoadOverridesFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	body := []byte("listen_address: :18080\nauth_token: secret\ngnocchi:\n  endpoint: http://gnocchi:8041\n  username: svc\n  password: pw\n  archive_policy_name: policy\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddress != ":18080" || cfg.AuthToken != "secret" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if cfg.Gnocchi.Endpoint != "http://gnocchi:8041" {
		t.Fatalf("Gnocchi.Endpoint = %q", cfg.Gnocchi.Endpoint)
	}
	if cfg.Gnocchi.Username != "svc" {
		t.Fatalf("Gnocchi.Username = %q", cfg.Gnocchi.Username)
	}
	if cfg.Gnocchi.Password != "pw" {
		t.Fatalf("Gnocchi.Password = %q", cfg.Gnocchi.Password)
	}
	if cfg.Gnocchi.ArchivePolicyName != "policy" {
		t.Fatalf("Gnocchi.ArchivePolicyName = %q", cfg.Gnocchi.ArchivePolicyName)
	}
}

func TestLoadRejectsMissingListenAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	body := []byte("listen_address: \"\"\ngnocchi:\n  endpoint: http://gnocchi:8041\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for missing listen address")
	}
}

func TestLoadRejectsMissingGnocchiEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(path, []byte("gnocchi:\n  endpoint: \"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for missing gnocchi endpoint")
	}
}
