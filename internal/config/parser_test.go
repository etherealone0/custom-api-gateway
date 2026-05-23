package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	// Setup temporary directory for test files
	dir := t.TempDir()

	tests := []struct {
		name        string
		yamlContent string
		envVars     map[string]string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "Valid Config with Env Expansion",
			yamlContent: `
server:
  port: 8080
routes:
  - path: /api
    backends:
      - url: http://localhost:9000
auth:
  jwt_secret: ${MY_JWT_SECRET}
`,
			envVars: map[string]string{
				"MY_JWT_SECRET": "super-secret-key",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 8080 {
					t.Errorf("expected port 8080, got %d", cfg.Server.Port)
				}
				if cfg.Auth.JWTSecret != "super-secret-key" {
					t.Errorf("expected expanded jwt_secret, got %s", cfg.Auth.JWTSecret)
				}
			},
		},
		{
			name: "Missing Server Port",
			yamlContent: `
server:
routes:
  - path: /api
    backends:
      - url: http://localhost:9000
`,
			wantErr:     true,
			errContains: "server.port is required",
		},
		{
			name: "Missing Routes",
			yamlContent: `
server:
  port: 8080
`,
			wantErr:     true,
			errContains: "at least one route is required",
		},
		{
			name: "Missing Route Path",
			yamlContent: `
server:
  port: 8080
routes:
  - backends:
      - url: http://localhost:9000
`,
			wantErr:     true,
			errContains: "routes[0].path is required",
		},
		{
			name: "Missing Backends",
			yamlContent: `
server:
  port: 8080
routes:
  - path: /api
`,
			wantErr:     true,
			errContains: "must have at least one backend",
		},
		{
			name: "Missing Backend URL",
			yamlContent: `
server:
  port: 8080
routes:
  - path: /api
    backends:
      - weight: 1
`,
			wantErr:     true,
			errContains: "routes[0].backends[0].url is required",
		},
		{
			name: "TLS Enabled but missing Certs",
			yamlContent: `
server:
  port: 8080
  tls:
    enabled: true
routes:
  - path: /api
    backends:
      - url: http://localhost:9000
`,
			wantErr:     true,
			errContains: "tls.cert_file and tls.key_file are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			filePath := filepath.Join(dir, "config.yaml")
			err := os.WriteFile(filePath, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(filePath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}
