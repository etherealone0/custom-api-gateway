package config

import "time"

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Routes        []RouteConfig       `yaml:"routes"`
	HealthCheck   HealthCheckConfig   `yaml:"health_check"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	Auth          AuthConfig          `yaml:"auth"`
	CORS          CORSConfig          `yaml:"cors"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Retry         RetryConfig         `yaml:"retry"`
}

// ServerConfig 
type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	TLS          TLSConfig     `yaml:"tls"`
}

// TLSConfig 
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// RouteConfig
type RouteConfig struct {
	Path        string          `yaml:"path"`
	StripPrefix bool            `yaml:"strip_prefix"`
	Strategy    string          `yaml:"strategy"`
	Backends    []BackendConfig `yaml:"backends"`
}

// BackendConfig
type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

// HealthCheckConfig
type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Path     string        `yaml:"path"`
}

// RateLimitConfig
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

// AuthConfig
type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"`
}

// CORSConfig 
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
}

// CircuitBreakerConfig
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
}

// RetryConfig
type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts"`
	BaseDelay   time.Duration `yaml:"base_delay"`
	MaxDelay    time.Duration `yaml:"max_delay"`
}
