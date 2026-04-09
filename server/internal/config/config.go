// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the server.
type Config struct {
	HTTP     HTTPConfig
	Database DatabaseConfig
	OTel     OTelConfig
	Log      LogConfig
	Auth     AuthConfig
}

// AuthConfig holds OIDC/Keycloak settings.
type AuthConfig struct {
	// OIDCIssuer is the issuer URL that appears in JWT `iss` claims.
	// For browser-based SPAs this is the external URL, e.g.
	// http://localhost:8080/realms/ai-registry
	OIDCIssuer string

	// OIDCJWKSUrl overrides the JWKS fetch URL. Set this to the internal
	// Docker hostname when the server cannot reach the external issuer URL,
	// e.g. http://keycloak:8080/realms/ai-registry/protocol/openid-connect/certs
	OIDCJWKSUrl string
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Addr             string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	CORSOrigins      []string
	// TrustedProxyCIDR is the optional CIDR (e.g. "10.0.0.0/8") of the
	// reverse proxy in front of this server. When set, X-Forwarded-For is
	// trusted for rate-limiting IP extraction. Parsed and stored as a string;
	// the caller parses it into *net.IPNet via net.ParseCIDR.
	TrustedProxyCIDR string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

// OTelConfig holds OpenTelemetry settings.
type OTelConfig struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string // empty = disable OTLP export, use Prometheus only
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string // debug, info, warn, error
}

// Load reads configuration from environment variables, applying defaults for
// any value not explicitly set.
func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr:             envString("HTTP_ADDR", ":8081"),
			ReadTimeout:      envDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:     envDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:      envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			CORSOrigins:      envStringSlice("CORS_ALLOWED_ORIGINS", nil),
			TrustedProxyCIDR: envString("TRUSTED_PROXY_CIDR", ""),
		},
		Database: DatabaseConfig{
			URL:      envString("DATABASE_URL", ""),
			MaxConns: int32(envInt("DATABASE_MAX_CONNS", 25)),
			MinConns: int32(envInt("DATABASE_MIN_CONNS", 5)),
		},
		OTel: OTelConfig{
			ServiceName:    envString("OTEL_SERVICE_NAME", "ai-registry-server"),
			ServiceVersion: envString("OTEL_SERVICE_VERSION", "0.1.0"),
			OTLPEndpoint:   envString("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		},
		Log: LogConfig{
			Level: envString("LOG_LEVEL", "info"),
		},
		Auth: AuthConfig{
			OIDCIssuer:  envString("OIDC_ISSUER", ""),
			OIDCJWKSUrl: envString("OIDC_JWKS_URL", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Auth.OIDCIssuer == "" {
		return fmt.Errorf("OIDC_ISSUER is required")
	}
	return nil
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}

func envStringSlice(key string, def []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return def
}
