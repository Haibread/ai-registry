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

	// OIDCInternalURL is the base URL (scheme://host) the server uses to reach
	// the IdP for back-channel calls — discovery, token exchange, JWKS — when the
	// public OIDCIssuer is unreachable from inside the network (e.g. Keycloak in
	// Docker: browser uses http://localhost:8080, server uses http://keycloak:8080).
	// The browser-facing authorize URL keeps the public host. Empty = use the
	// public issuer for everything. The IdP must advertise OIDCIssuer (pin its
	// hostname, e.g. Keycloak KC_HOSTNAME) so `iss` and the authorize URL match.
	OIDCInternalURL string

	// OIDCClientID is the confidential OAuth 2.0 client ID the server-side
	// broker uses. It is NOT served to the
	// browser — the SPA is no longer an OIDC client.
	OIDCClientID string

	// OIDCClientSecret is the confidential client's secret for the server-side
	// broker. A credential — supply via env or a secrets manager, never a
	// committed config file. Required to enable OIDC login.
	OIDCClientSecret string

	// OIDCRedirectURL is the callback the IdP redirects to after login. When
	// empty it is derived from PublicBaseURL + /api/v1/auth/oidc/callback.
	OIDCRedirectURL string

	// OIDCScopes are requested at the authorize endpoint. Empty defaults to
	// openid / profile / email.
	OIDCScopes []string

	// GroupsClaim is the JWT payload key the validator reads group
	// memberships from. Default "groups". Configurable via env+YAML+
	// default per CLAUDE.md when an external IdP emits this claim
	// under a different name.
	GroupsClaim string

	// ReviewerGroup is the group seeded on boot with a global Reviewer
	// grant. Retained for back-compat: on boot the server
	// ensures a group of this name exists with a global (publisher_id NULL)
	// Reviewer grant (source = config). Deprecated in favour of managing the
	// group + grant via the API. Default: "registry-reviewers". Configurable
	// via env + YAML + default per CLAUDE.md's configuration rule.
	ReviewerGroup string

	// LocalLoginEnabled turns the local email+password front door on or off
	// Default true so the registry is usable without an
	// external IdP. Both front doors end in a registry session cookie, so no
	// token-signing key is needed.
	LocalLoginEnabled bool

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

	// SessionCookieName is the name of the HttpOnly session cookie set after a
	// successful login. Default
	// "ai_registry_session".
	SessionCookieName string

	// SessionTTL is how long a login session is valid. Fixed in v1 (no sliding
	// / refresh), so users re-login at expiry. Default 24h.
	SessionTTL time.Duration

	// SessionCookieSecure sets the cookie's Secure flag. Default true; set
	// false only for local plain-HTTP development.
	SessionCookieSecure bool

	// SessionCookieSameSite is the cookie SameSite policy: "lax" (default),
	// "strict", or "none".
	SessionCookieSameSite string
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
	OIDCIssuer      string   `yaml:"oidc_issuer"`
	OIDCJWKSUrl     string   `yaml:"oidc_jwks_url"`
	OIDCInternalURL string   `yaml:"oidc_internal_url"`
	OIDCClientID    string   `yaml:"oidc_client_id"`
	OIDCRedirectURL string   `yaml:"oidc_redirect_url"`
	OIDCScopes      []string `yaml:"oidc_scopes"`
	// oidc_client_secret is a credential — env / secret only, never the file.
	GroupsClaim   string `yaml:"groups_claim"`
	ReviewerGroup string `yaml:"reviewer_group"`
	LocalLogin    bool   `yaml:"local_login"`
	// The bootstrap admin password is a credential — env / secret only, never
	// read from the config file (secrets conventions). The bootstrap admin
	// EMAIL is not a secret, so it may be set in the file.
	BootstrapAdminEmail string `yaml:"bootstrap_admin_email"`
	SessionCookieName   string `yaml:"session_cookie_name"`
	SessionTTL          string `yaml:"session_ttl"`
	SessionSecure       bool   `yaml:"session_secure"`
	SessionSameSite     string `yaml:"session_samesite"`
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
			GroupsClaim:       "groups",
			ReviewerGroup:     "registry-reviewers",
			LocalLogin:        true,
			SessionCookieName: "ai_registry_session",
			SessionTTL:        "24h",
			SessionSecure:     true,
			SessionSameSite:   "lax",
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
	sessionTTL := parseDurationDefault(fc.Auth.SessionTTL, 24*time.Hour)

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
			OIDCInternalURL:        envString("OIDC_INTERNAL_URL", fc.Auth.OIDCInternalURL),
			OIDCClientID:           envString("OIDC_CLIENT_ID", fc.Auth.OIDCClientID),
			OIDCClientSecret:       envString("OIDC_CLIENT_SECRET", ""),
			OIDCRedirectURL:        envString("OIDC_REDIRECT_URL", fc.Auth.OIDCRedirectURL),
			OIDCScopes:             envStringSlice("OIDC_SCOPES", fc.Auth.OIDCScopes),
			GroupsClaim:            envString("AUTH_GROUPS_CLAIM", fc.Auth.GroupsClaim),
			ReviewerGroup:          envString("AUTH_REVIEWER_GROUP", fc.Auth.ReviewerGroup),
			LocalLoginEnabled:      envBool("AUTH_LOCAL_LOGIN_ENABLED", fc.Auth.LocalLogin),
			BootstrapAdminEmail:    envString("AUTH_BOOTSTRAP_ADMIN_EMAIL", fc.Auth.BootstrapAdminEmail),
			BootstrapAdminPassword: envString("AUTH_BOOTSTRAP_ADMIN_PASSWORD", ""),
			SessionCookieName:      envString("AUTH_SESSION_COOKIE_NAME", fc.Auth.SessionCookieName),
			SessionTTL:             envDuration("AUTH_SESSION_TTL", sessionTTL),
			SessionCookieSecure:    envBool("AUTH_SESSION_SECURE", fc.Auth.SessionSecure),
			SessionCookieSameSite:  envString("AUTH_SESSION_SAMESITE", fc.Auth.SessionSameSite),
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

	// OIDC is brokered only when a confidential client is fully configured
	// (both ID and secret). Mirror cmd/server's enablement check so validation
	// and runtime agree on when OIDC is "on". A half-configured client is almost
	// always a mistake that would otherwise silently leave OIDC disabled.
	hasClientID := c.Auth.OIDCClientID != ""
	hasClientSecret := c.Auth.OIDCClientSecret != ""
	if hasClientID != hasClientSecret {
		return fmt.Errorf("OIDC is half-configured: set both OIDC_CLIENT_ID and OIDC_CLIENT_SECRET to enable OIDC, or neither to disable it")
	}
	oidcEnabled := hasClientID && hasClientSecret

	// OIDC_ISSUER is only needed when OIDC is enabled — the broker validates the
	// id_token `iss` against it. Local-login-only deployments run without an IdP
	// (decision N), so an empty issuer is valid when OIDC is off.
	if oidcEnabled && c.Auth.OIDCIssuer == "" {
		return fmt.Errorf("OIDC_ISSUER is required when OIDC is enabled (OIDC_CLIENT_ID and OIDC_CLIENT_SECRET are set)")
	}

	// At least one front door must be open, or no one can log in.
	if !oidcEnabled && !c.Auth.LocalLoginEnabled {
		return fmt.Errorf("no login method enabled: set AUTH_LOCAL_LOGIN_ENABLED=true, or configure OIDC (OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET)")
	}

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
