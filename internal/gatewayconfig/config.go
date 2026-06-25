package gatewayconfig

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddress string        `yaml:"listen_address"`
	AuthToken     string        `yaml:"auth_token"`
	Gnocchi       GnocchiConfig `yaml:"gnocchi"`
}

type GnocchiConfig struct {
	Endpoint          string `yaml:"endpoint"`
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	ArchivePolicyName string `yaml:"archive_policy_name"`
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
	if cfg.ListenAddress == "" {
		return Config{}, errors.New("listen_address must not be empty")
	}
	if cfg.Gnocchi.Endpoint == "" {
		return Config{}, errors.New("gnocchi.endpoint must not be empty")
	}
	if cfg.Gnocchi.Username == "" {
		cfg.Gnocchi.Username = "admin"
	}
	if cfg.Gnocchi.ArchivePolicyName == "" {
		cfg.Gnocchi.ArchivePolicyName = "vm_guest_usage_default"
	}
	return cfg, nil
}

func Defaults() Config {
	return Config{
		ListenAddress: ":8080",
		Gnocchi: GnocchiConfig{
			Endpoint:          "http://127.0.0.1:8041",
			Username:          "admin",
			ArchivePolicyName: "vm_guest_usage_default",
		},
	}
}
