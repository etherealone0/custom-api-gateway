package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Reads YAML config
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// Config Validation
func validate(cfg *Config) error {
	if cfg.Server.Port == 0 {
		return fmt.Errorf("server.port is required")
	}

	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}

	for i, route := range cfg.Routes {
		if route.Path == "" {
			return fmt.Errorf("routes[%d].path is required", i)
		}
		if len(route.Backends) == 0 {
			return fmt.Errorf("routes[%d] (%s) must have at least one backend", i, route.Path)
		}
		for j, backend := range route.Backends {
			if backend.URL == "" {
				return fmt.Errorf("routes[%d].backends[%d].url is required", i, j)
			}
		}
	}

	if cfg.Server.TLS.Enabled {
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("tls.cert_file and tls.key_file are required when tls is enabled")
		}
	}

	return nil
}
