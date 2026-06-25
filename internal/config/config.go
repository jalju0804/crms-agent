package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultMetadataURL = "http://169.254.169.254/openstack/latest/meta_data.json"

const (
	OutputModeJSONL   = "jsonl"
	OutputModeGnocchi = "gnocchi"
	OutputModeBoth    = "both"
	OutputModeGateway = "gateway"
)

type Config struct {
	Interval               time.Duration `yaml:"interval"`
	OutputMode             string        `yaml:"output_mode"`
	OutputPath             string        `yaml:"output_path"`
	RootPath               string        `yaml:"root_path"`
	MetadataURL            string        `yaml:"metadata_url"`
	Gnocchi                GnocchiConfig `yaml:"gnocchi"`
	Gateway                GatewayConfig `yaml:"gateway"`
	NetworkExcludePrefixes []string      `yaml:"network_exclude_prefixes"`
	DiskExcludePrefixes    []string      `yaml:"disk_exclude_prefixes"`
}

type GnocchiConfig struct {
	Endpoint          string `yaml:"endpoint"`
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	ArchivePolicyName string `yaml:"archive_policy_name"`
}

type GatewayConfig struct {
	Endpoint string `yaml:"endpoint"`
	Token    string `yaml:"token"`
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path != "" {
		body, err := os.ReadFile(path)
		if err != nil {
			return Config{}, err
		}
		if err := yaml.Unmarshal(body, &cfg); err != nil {
			return Config{}, err
		}
	}
	if cfg.Interval <= 0 {
		return Config{}, errors.New("interval must be positive")
	}
	if cfg.OutputPath == "" {
		return Config{}, errors.New("output_path must not be empty")
	}
	switch cfg.OutputMode {
	case OutputModeJSONL, OutputModeGnocchi, OutputModeBoth, OutputModeGateway:
	default:
		return Config{}, errors.New("output_mode must be jsonl, gnocchi, both, or gateway")
	}
	if (cfg.OutputMode == OutputModeGnocchi || cfg.OutputMode == OutputModeBoth) && cfg.Gnocchi.Endpoint == "" {
		return Config{}, errors.New("gnocchi.endpoint must not be empty when output_mode uses gnocchi")
	}
	if cfg.OutputMode == OutputModeGateway && cfg.Gateway.Endpoint == "" {
		return Config{}, errors.New("gateway.endpoint must not be empty when output_mode is gateway")
	}
	if cfg.Gnocchi.Username == "" {
		cfg.Gnocchi.Username = "admin"
	}
	if cfg.Gnocchi.ArchivePolicyName == "" {
		cfg.Gnocchi.ArchivePolicyName = "vm_guest_usage_default"
	}
	if cfg.RootPath == "" {
		return Config{}, errors.New("root_path must not be empty")
	}
	if cfg.MetadataURL == "" {
		cfg.MetadataURL = DefaultMetadataURL
	}
	return cfg, nil
}

func Defaults() Config {
	return Config{
		Interval:    time.Minute,
		OutputMode:  OutputModeJSONL,
		OutputPath:  "/var/lib/vm-metric-agent/samples.jsonl",
		RootPath:    "/",
		MetadataURL: DefaultMetadataURL,
		Gnocchi: GnocchiConfig{
			Username:          "admin",
			ArchivePolicyName: "vm_guest_usage_default",
		},
		NetworkExcludePrefixes: []string{
			"lo",
			"veth",
			"cni",
			"flannel",
			"cilium",
			"docker",
			"br-",
		},
		DiskExcludePrefixes: []string{
			"loop",
			"ram",
			"fd",
			"sr",
		},
	}
}
