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

	// BootstrapFile is the optional path to a YAML/JSON file containing
	// initial registry data (publishers, MCP servers, agents)
	// that the server upserts on startup before accepting traffic. Empty
	// disables bootstrap loading. Settable via env (BOOTSTRAP_FILE), YAML
	// (top-level `bootstrap_file`), or the `--bootstrap-file` CLI flag —
	// the flag wins over both. See `deploy/bootstrap.example.yaml`.
	BootstrapFile string
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

	// OIDCAudience is the expected `aud` value on incoming access tokens.
	// When non-empty, tokens missing this audience are rejected — required by
	// the MCP authorization spec (OAuth 2.1 resource indicators / audience
	// binding) to prevent tokens minted for unrelated clients on the same
	// realm from being accepted at this resource server.
	OIDCAudience string

	// GroupsClaim is the JWT payload key the validator reads group
	// memberships from. Default "groups". Configurable via env+YAML+
	// default per CLAUDE.md when an external IdP emits this claim
	// under a different name.
	GroupsClaim string

	// AuthStorage controls where the browser SPA persists OIDC tokens.
	// "session" (the default) scopes tokens to the browser tab, limiting
	// XSS-exfiltration blast radius. "local" is an E2E escape hatch —
	// Playwright's storageState() captures localStorage across contexts, so
	// the e2e stack sets AUTH_STORAGE=local. Never use "local" in production.
	AuthStorage string

	// ReviewerGroup is the group seeded on boot with a global Reviewer
	// grant (ADR 0006 §5). Retained for back-compat: on boot the server
	// ensures a group of this name exists with a global (publisher_id NULL)
	// Reviewer grant (source = config). Deprecated in favour of managing the
	// group + grant via the API. Default: "registry-reviewers". Configurable
	// via env + YAML + default per CLAUDE.md's configuration rule.
	ReviewerGroup string

	// LocalLoginEnabled turns the local email+password front door on or off
	// (ADR 0006 §3). When true, LocalSigningKey is required. Default true so
	// the registry is usable without an external IdP.
	LocalLoginEnabled bool

	// LocalSigningKey is the PEM-encoded private key (RSA or Ed25519) the
	// registry uses to sign its own access tokens for local logins and to
	// publish a self-verification JWKS. It is a credential — supply via env
	// or a secrets manager, never a committed config file. Required when
	// LocalLoginEnabled is true.
	LocalSigningKey string

	// LocalTokenTTL is how long a registry-issued local token is valid.
	// Default 1h; local users re-login at expiry (no refresh tokens in v1).
	LocalTokenTTL time.Duration

	// BootstrapAdminEmail is the email of the local Server Admin seeded on
	// first boot (is_server_admin = true). Empty disables bootstrap seeding.
	// The seed is create-only: it never overwrites an existing account's
	// password, so a rotated password survives reboots.
	BootstrapAdminEmail string

	// BootstrapAdminPassword is the initial password for the bootstrap admin.
	// It is a credential — env / secret only, never a config file. Consumed
	// at first boot and should be rotated. Required when BootstrapAdminEmail
	// is set.
	BootstrapAdminPassword string
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	CORSOrigins  []string
	// TrustedProxyCIDR is the optional CIDR (e.g. "10.0.0.0/8") of the
	// reverse proxy in front of this server. When set, X-Forwarded-For is
	// trusted for rate-limiting IP extraction. Parsed and stored as a string;
	// the caller parses it into *net.IPNet via net.ParseCIDR.
	TrustedProxyCIDR string
	// PublicRateLimitRPM is the per-IP request budget for unauthenticated
	// reads on /api/v1, expressed in requests per minute. Defaults to 1000.
	// Bumped from the original 100 because the e2e suite + browser-based
	// SPAs can comfortably exceed the lower bound under normal use.
	PublicRateLimitRPM int
	// PublicBaseURL is the externally reachable URL of this deployment.
	// Surfaced in the A2A global agent card (`/.well-known/agent-card.json`)
	// and the OAuth protected-resource metadata document
	// (`/.well-known/oauth-protected-resource`). Must be the address clients
	// use, not an internal docker hostname. Handlers return 500 when this
	// is empty rather than silently advertising localhost to external
	// consumers.
	PublicBaseURL string
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
	Addr               string   `yaml:"addr"`
	ReadTimeout        string   `yaml:"read_timeout"`
	WriteTimeout       string   `yaml:"write_timeout"`
	IdleTimeout        string   `yaml:"idle_timeout"`
	CORSOrigins        []string `yaml:"cors_origins"`
	TrustedProxyCIDR   string   `yaml:"trusted_proxy_cidr"`
	PublicRateLimitRPM int      `yaml:"public_rate_limit_rpm"`
	PublicBaseURL      string   `yaml:"public_base_url"`
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
	OIDCIssuer    string `yaml:"oidc_issuer"`
	OIDCJWKSUrl   string `yaml:"oidc_jwks_url"`
	OIDCClientID  string `yaml:"oidc_client_id"`
	OIDCAudience  string `yaml:"oidc_audience"`
	AuthStorage   string `yaml:"auth_storage"`
	GroupsClaim   string `yaml:"groups_claim"`
	ReviewerGroup string `yaml:"reviewer_group"`
	LocalLogin    bool   `yaml:"local_login"`
	// local_signing_key and the bootstrap admin password are credentials —
	// env / secret only, never read from the config file (secrets
	// conventions). The bootstrap admin EMAIL is not a secret, so it may be
	// set in the file.
	LocalTokenTTL       string `yaml:"local_token_ttl"`
	BootstrapAdminEmail string `yaml:"bootstrap_admin_email"`
}

type fileConfig struct {
	HTTP          fileHTTPConfig     `yaml:"http"`
	Database      fileDatabaseConfig `yaml:"database"`
	OTel          fileOTelConfig     `yaml:"otel"`
	Log           fileLogConfig      `yaml:"log"`
	Auth          fileAuthConfig     `yaml:"auth"`
	BootstrapFile string             `yaml:"bootstrap_file"`
}

// defaultFileConfig returns a fileConfig pre-populated with the same defaults
// that Load uses, so absent keys in the YAML file keep their defaults.
func defaultFileConfig() fileConfig {
	return fileConfig{
		HTTP: fileHTTPConfig{
			Addr:               ":8081",
			ReadTimeout:        "30s",
			WriteTimeout:       "30s",
			IdleTimeout:        "120s",
			PublicRateLimitRPM: 1000,
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
		Auth: fileAuthConfig{
			GroupsClaim:   "groups",
			ReviewerGroup: "registry-reviewers",
			LocalLogin:    true,
			LocalTokenTTL: "1h",
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
	localTokenTTL := parseDurationDefault(fc.Auth.LocalTokenTTL, time.Hour)

	// Build final config: env vars win over file values.
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr:               envString("HTTP_ADDR", fc.HTTP.Addr),
			ReadTimeout:        envDuration("HTTP_READ_TIMEOUT", readTimeout),
			WriteTimeout:       envDuration("HTTP_WRITE_TIMEOUT", writeTimeout),
			IdleTimeout:        envDuration("HTTP_IDLE_TIMEOUT", idleTimeout),
			CORSOrigins:        envStringSlice("CORS_ALLOWED_ORIGINS", fc.HTTP.CORSOrigins),
			TrustedProxyCIDR:   envString("TRUSTED_PROXY_CIDR", fc.HTTP.TrustedProxyCIDR),
			PublicRateLimitRPM: envInt("PUBLIC_RATE_LIMIT_RPM", fc.HTTP.PublicRateLimitRPM),
			PublicBaseURL:      envString("PUBLIC_BASE_URL", fc.HTTP.PublicBaseURL),
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
			OIDCIssuer:             envString("OIDC_ISSUER", fc.Auth.OIDCIssuer),
			OIDCJWKSUrl:            envString("OIDC_JWKS_URL", fc.Auth.OIDCJWKSUrl),
			OIDCClientID:           envString("OIDC_CLIENT_ID", fc.Auth.OIDCClientID),
			OIDCAudience:           envString("OIDC_AUDIENCE", fc.Auth.OIDCAudience),
			AuthStorage:            envString("AUTH_STORAGE", fc.Auth.AuthStorage),
			GroupsClaim:            envString("AUTH_GROUPS_CLAIM", fc.Auth.GroupsClaim),
			ReviewerGroup:          envString("AUTH_REVIEWER_GROUP", fc.Auth.ReviewerGroup),
			LocalLoginEnabled:      envBool("AUTH_LOCAL_LOGIN_ENABLED", fc.Auth.LocalLogin),
			LocalSigningKey:        envString("AUTH_LOCAL_SIGNING_KEY", ""),
			LocalTokenTTL:          envDuration("AUTH_LOCAL_TOKEN_TTL", localTokenTTL),
			BootstrapAdminEmail:    envString("AUTH_BOOTSTRAP_ADMIN_EMAIL", fc.Auth.BootstrapAdminEmail),
			BootstrapAdminPassword: envString("AUTH_BOOTSTRAP_ADMIN_PASSWORD", ""),
		},
		BootstrapFile: envString("BOOTSTRAP_FILE", fc.BootstrapFile),
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
	defer func() { _ = f.Close() }()

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
	// Audience binding is non-negotiable for the OAuth / MCP surface (OAuth 2.1
	// resource indicators). With an empty audience the validator skips the `aud`
	// check and accepts any token the realm signed for any client, so we fail
	// closed at boot rather than silently allow cross-client token reuse.
	if c.Auth.OIDCAudience == "" {
		return fmt.Errorf("OIDC_AUDIENCE is required (OAuth 2.1 audience binding; set it to this resource server's audience, e.g. ai-registry-server)")
	}
	// The "AUTH_LOCAL_SIGNING_KEY required when local login is enabled" rule is
	// enforced where the local-auth subsystem initialises (it is the component
	// that actually needs the key and can fail closed without a half-built
	// feature breaking unrelated config loads). LocalLoginEnabled defaults to
	// true, so validating the key here would reject every deployment until the
	// local-auth wiring lands.
	if c.Auth.BootstrapAdminEmail != "" && c.Auth.BootstrapAdminPassword == "" {
		return fmt.Errorf("AUTH_BOOTSTRAP_ADMIN_PASSWORD is required when AUTH_BOOTSTRAP_ADMIN_EMAIL is set")
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

// envBool reads a boolean env var, falling back to def when unset or
// unparseable. Accepts the forms strconv.ParseBool understands
// (1/t/T/TRUE/true/0/f/F/FALSE/false, …). Needed for knobs whose default is
// true, where an explicit "false" must override the default.
func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
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
