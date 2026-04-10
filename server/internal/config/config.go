// Package config loads application configuration from environment variables,
// an optional YAML config file, and built-in defaults. Precedence (highest
// first): environment variable > YAML file > built-in default.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

	// OIDCClientID is the public OAuth 2.0 client ID for the browser SPA.
	// Served via GET /config.json so the frontend can bootstrap its OIDC
	// client at runtime without baking the value into the Docker image.
	OIDCClientID string
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

// ── YAML file types ──────────────────────────────────────────────────────────
// These mirror Config but use string durations so the YAML file can express
// them as "30s", "2m", etc.  Fields that are absent in the YAML file keep
// whatever value was pre-populated (the built-in default).

type fileHTTPConfig struct {
	Addr             string   `yaml:"addr"`
	ReadTimeout      string   `yaml:"read_timeout"`
	WriteTimeout     string   `yaml:"write_timeout"`
	IdleTimeout      string   `yaml:"idle_timeout"`
	CORSOrigins      []string `yaml:"cors_origins"`
	TrustedProxyCIDR string   `yaml:"trusted_proxy_cidr"`
}

type fileDatabaseConfig struct {
	URL      string `yaml:"url"`
	MaxConns int    `yaml:"max_conns"`
	MinConns int    `yaml:"min_conns"`
}

type fileOTelConfig struct {
	ServiceName    string `yaml:"service_name"`
	ServiceVersion string `yaml:"service_version"`
	OTLPEndpoint   string `yaml:"otlp_endpoint"`
}

type fileLogConfig struct {
	Level string `yaml:"level"`
}

type fileAuthConfig struct {
	OIDCIssuer   string `yaml:"oidc_issuer"`
	OIDCJWKSUrl  string `yaml:"oidc_jwks_url"`
	OIDCClientID string `yaml:"oidc_client_id"`
}

type fileConfig struct {
	HTTP     fileHTTPConfig     `yaml:"http"`
	Database fileDatabaseConfig `yaml:"database"`
	OTel     fileOTelConfig     `yaml:"otel"`
	Log      fileLogConfig      `yaml:"log"`
	Auth     fileAuthConfig     `yaml:"auth"`
}

// defaultFileConfig returns a fileConfig pre-populated with the same defaults
// that Load uses, so absent keys in the YAML file keep their defaults.
func defaultFileConfig() fileConfig {
	return fileConfig{
		HTTP: fileHTTPConfig{
			Addr:         ":8081",
			ReadTimeout:  "30s",
			WriteTimeout: "30s",
			IdleTimeout:  "120s",
		},
		Database: fileDatabaseConfig{
			MaxConns: 25,
			MinConns: 5,
		},
		OTel: fileOTelConfig{
			ServiceName:    "ai-registry-server",
			ServiceVersion: "0.1.0",
		},
		Log: fileLogConfig{
			Level: "info",
		},
	}
}

// Load reads configuration using three-layer precedence:
//
//  1. Environment variables (highest priority)
//  2. YAML config file — path resolved from configFile argument, then
//     the CONFIG_FILE environment variable.  Missing file is not an error.
//  3. Built-in defaults (lowest priority)
//
// Pass an empty string for configFile to rely solely on CONFIG_FILE or
// defaults.
func Load(configFile string) (*Config, error) {
	// Resolve config file path.
	if configFile == "" {
		configFile = os.Getenv("CONFIG_FILE")
	}

	// Start from built-in defaults.
	fc := defaultFileConfig()

	// Overlay with YAML file (if any).
	if configFile != "" {
		if err := loadFile(configFile, &fc); err != nil {
			return nil, err
		}
	}

	// Parse durations from file config (already defaulted above).
	readTimeout := parseDurationDefault(fc.HTTP.ReadTimeout, 30*time.Second)
	writeTimeout := parseDurationDefault(fc.HTTP.WriteTimeout, 30*time.Second)
	idleTimeout := parseDurationDefault(fc.HTTP.IdleTimeout, 120*time.Second)

	// Build final config: env vars win over file values.
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr:             envString("HTTP_ADDR", fc.HTTP.Addr),
			ReadTimeout:      envDuration("HTTP_READ_TIMEOUT", readTimeout),
			WriteTimeout:     envDuration("HTTP_WRITE_TIMEOUT", writeTimeout),
			IdleTimeout:      envDuration("HTTP_IDLE_TIMEOUT", idleTimeout),
			CORSOrigins:      envStringSlice("CORS_ALLOWED_ORIGINS", fc.HTTP.CORSOrigins),
			TrustedProxyCIDR: envString("TRUSTED_PROXY_CIDR", fc.HTTP.TrustedProxyCIDR),
		},
		Database: DatabaseConfig{
			URL:      envString("DATABASE_URL", fc.Database.URL),
			MaxConns: int32(envInt("DATABASE_MAX_CONNS", fc.Database.MaxConns)),
			MinConns: int32(envInt("DATABASE_MIN_CONNS", fc.Database.MinConns)),
		},
		OTel: OTelConfig{
			ServiceName:    envString("OTEL_SERVICE_NAME", fc.OTel.ServiceName),
			ServiceVersion: envString("OTEL_SERVICE_VERSION", fc.OTel.ServiceVersion),
			OTLPEndpoint:   envString("OTEL_EXPORTER_OTLP_ENDPOINT", fc.OTel.OTLPEndpoint),
		},
		Log: LogConfig{
			Level: envString("LOG_LEVEL", fc.Log.Level),
		},
		Auth: AuthConfig{
			OIDCIssuer:   envString("OIDC_ISSUER", fc.Auth.OIDCIssuer),
			OIDCJWKSUrl:  envString("OIDC_JWKS_URL", fc.Auth.OIDCJWKSUrl),
			OIDCClientID: envString("OIDC_CLIENT_ID", fc.Auth.OIDCClientID),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadFile reads a YAML file into fc. fc must be pre-populated with defaults;
// only keys present in the file are overwritten. Returns nil if the file does
// not exist.
func loadFile(path string, fc *fileConfig) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: open %q: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // reject unknown keys to catch typos
	if err := dec.Decode(fc); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("config: parse %q: %w", path, err)
	}
	return nil
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

// ── env helpers ───────────────────────────────────────────────────────────────

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

// parseDurationDefault parses s as a duration; returns def on parse failure.
func parseDurationDefault(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
