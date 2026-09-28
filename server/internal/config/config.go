// Package config loads application configuration from environment variables,
// an optional YAML config file, and built-in defaults. Precedence (highest
// first): environment variable > YAML file > built-in default.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/haibread/ai-registry/internal/domain"
)

// Config holds all runtime configuration for the server.
type Config struct {
	HTTP     HTTPConfig
	Database DatabaseConfig
	OTel     OTelConfig
	Log      LogConfig
	Auth     AuthConfig

	ToolDiscovery ToolDiscoveryConfig

	// BootstrapFile is the optional path to a YAML/JSON file containing
	// initial registry data (publishers, MCP servers, agents)
	// that the server upserts on startup before accepting traffic. Empty
	// disables bootstrap loading. Settable via env (BOOTSTRAP_FILE), YAML
	// (top-level `bootstrap_file`), or the `--bootstrap-file` CLI flag —
	// the flag wins over both. See `deploy/bootstrap.example.yaml`.
	BootstrapFile string

	// InstanceTags is the config-managed portion of the instance-wide tag
	// vocabulary. Reconciled into the database at every startup: each listed
	// tag is created or updated (and flagged managed → read-only via the
	// API), and previously managed tags that are no longer listed are
	// released to admin-UI ownership. Settable via env (INSTANCE_TAGS, a
	// JSON array) or YAML (top-level `instance_tags`); default empty (the
	// vocabulary is then curated solely through the admin API/UI).
	InstanceTags []InstanceTagSpec
}

// InstanceTagSpec is one entry of the config-managed tag vocabulary. The
// YAML config file (`instance_tags:`) and the INSTANCE_TAGS env var (a JSON
// array — a structured list cannot be a flat scalar) both decode into it.
type InstanceTagSpec struct {
	Slug        string `yaml:"slug"        json:"slug"`
	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description" json:"description"`
	// Color defaults to "gray" when omitted.
	Color string `yaml:"color" json:"color"`
	// Active defaults to true when omitted; set false to ship the tag
	// deactivated (visible on old versions, not tickable on new ones).
	Active *bool `yaml:"active" json:"active"`
}

// ToolDiscoveryConfig bounds the outbound connections the registry makes to
// list a remote MCP server's tools on an author's behalf.
type ToolDiscoveryConfig struct {
	// Enabled turns POST /api/v1/mcp/tool-discoveries on. When false the
	// route answers 503.
	Enabled bool
	// Timeout is the budget for one discovery, every endpoint guess included.
	Timeout time.Duration
	// AllowedCIDRs exempts ranges from the block on loopback, private,
	// link-local and other reserved addresses, for MCP servers hosted on an
	// internal network.
	AllowedCIDRs []netip.Prefix
	// MaxTools caps the number of tools one discovery reads.
	MaxTools int
	// RateLimitRPM is the per-IP budget of discoveries, per minute.
	RateLimitRPM int
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
	// broker uses. It is never served to the browser — the SPA is not an OIDC
	// client.
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

	// EmailClaim is the JWT payload key the broker reads the user's email from.
	// Default "email". The email claim name is not guaranteed across IdPs, so it
	// is configurable via env + YAML + default per CLAUDE.md (mirrors
	// GroupsClaim); dotted paths address nested objects.
	EmailClaim string

	// ReviewerGroup is the group seeded on boot with a global Reviewer
	// grant. Retained for back-compat: on boot the server
	// ensures a group of this name exists with a global (publisher_id NULL)
	// Reviewer grant (source = config). Deprecated in favour of managing the
	// group + grant via the API. Default: "registry-reviewers". Configurable
	// via env + YAML + default per CLAUDE.md's configuration rule.
	ReviewerGroup string

	// RolesClaim is the JWT payload path the broker reads realm/global roles
	// from when mapping an OIDC identity. Dotted paths address nested objects
	// (e.g. "realm_access.roles"). Default "realm_access.roles". Configurable via
	// env + YAML + default per CLAUDE.md.
	RolesClaim string

	// AdminRole is the role value that, when present in RolesClaim, grants the
	// Server Admin flag snapshotted into the registry JWT at login. Default
	// "admin".
	AdminRole string

	// OIDCAudience enables direct acceptance of IdP-issued (e.g. Keycloak
	// service-account) access tokens on the API, for machine-to-machine callers
	// that cannot run the interactive browser login (a Kubernetes operator, a CI
	// job). When set, a bearer that is not a registry token is verified offline
	// as an IdP JWT against the broker JWKS, and accepted only if its `aud`
	// contains this exact value — so a token minted for another client in the
	// same realm is never honoured (the audience pin is the security boundary).
	// The mapped realm-admin role grants Server Admin and the `groups` claim
	// drives publisher-scoped RBAC, exactly as a brokered login. Empty (the
	// default) disables the path entirely: only registry-issued tokens are
	// accepted, which keeps the IdP-less / break-glass deployment unchanged.
	// Requires the OIDC broker to be configured (OIDC_CLIENT_ID/SECRET).
	OIDCAudience string

	// LocalLoginEnabled turns the local email+password front door on or off
	// Default true so the registry is usable without an external IdP. Both front
	// doors mint a registry-issued JWT (no cookie); see JWTSigningKey.
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

	// JWTSigningKey is the PEM-encoded Ed25519 private key (PKCS#8) the server
	// signs registry access tokens with. CREDENTIAL — supply via env or a
	// secrets manager, never a committed config file. Takes precedence over
	// JWTSigningSeed. Empty (and no seed) triggers an ephemeral key generated at
	// boot (dev only): tokens then do not survive a restart and cannot be
	// verified by other replicas. Set one of the two in any real deployment.
	JWTSigningKey string

	// JWTSigningSeed is an arbitrary high-entropy secret string the server
	// derives the Ed25519 signing key from deterministically (same seed → same
	// key across replicas and restarts). A convenient alternative to managing a
	// PEM: set one secret string. CREDENTIAL — env / secrets manager only.
	// Ignored when JWTSigningKey is set. Must be high-entropy and is
	// length-checked at boot (see auth.minSigningSeedLen).
	JWTSigningSeed string

	// AccessTokenTTL is how long a registry access token is valid. Short by
	// design (the refresh token carries longevity). Default 15m.
	AccessTokenTTL time.Duration

	// RefreshTokenTTL is the absolute session lifetime from login: rotated
	// refresh tokens keep the original expiry, so the user must re-authenticate
	// once it elapses. Default 12h. Kept short because OIDC group and admin
	// claims are snapshotted at login and only re-read on a fresh login, so this
	// bounds how long a revoked IdP membership keeps conferring roles.
	RefreshTokenTTL time.Duration

	// SweepInterval is how often the server purges expired refresh tokens,
	// OIDC login transactions and handoff codes. Default 15m.
	SweepInterval time.Duration
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	CORSOrigins  []string
	// TrustedProxyCIDR is the optional CIDR (e.g. "10.0.0.0/8") of the
	// reverse proxies in front of this server. When set, the client IP is the
	// rightmost X-Forwarded-For hop outside it. Parsed and stored as a string;
	// the caller parses it into *net.IPNet via net.ParseCIDR.
	TrustedProxyCIDR string
	// PublicRateLimitRPM is the per-IP request budget for unauthenticated
	// reads on /api/v1, expressed in requests per minute. Defaults to 1000,
	// which a browser SPA and the e2e suite stay under in normal use.
	PublicRateLimitRPM int
	// PublicBaseURL is the externally reachable URL of this deployment.
	// Surfaced in the A2A global agent card (`/.well-known/agent-card.json`),
	// used as the access-token issuer and to derive the OIDC callback and
	// post-login/logout redirects. Must be the address clients use, not an
	// internal docker hostname. The global agent card returns 500 when this
	// is empty rather than silently advertising localhost.
	PublicBaseURL string
	// ShutdownDrainDelay is how long the server keeps serving after SIGTERM,
	// with /readyz failing, before it stops accepting connections. It gives
	// load balancers time to stop routing here. Zero disables the delay.
	ShutdownDrainDelay time.Duration
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
	ShutdownDrainDelay string   `yaml:"shutdown_drain_delay"`
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
	EmailClaim    string `yaml:"email_claim"`
	RolesClaim    string `yaml:"oidc_roles_claim"`
	AdminRole     string `yaml:"oidc_admin_role"`
	OIDCAudience  string `yaml:"oidc_audience"`
	ReviewerGroup string `yaml:"reviewer_group"`
	LocalLogin    bool   `yaml:"local_login"`
	// The bootstrap admin password is a credential — env / secret only, never
	// read from the config file (secrets conventions). The bootstrap admin
	// EMAIL is not a secret, so it may be set in the file.
	BootstrapAdminEmail string `yaml:"bootstrap_admin_email"`
	// jwt_signing_key is a credential — env / secret only, never the file.
	AccessTokenTTL  string `yaml:"access_token_ttl"`
	RefreshTokenTTL string `yaml:"refresh_token_ttl"`
	SweepInterval   string `yaml:"sweep_interval"`
}

type fileToolDiscoveryConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Timeout      string   `yaml:"timeout"`
	AllowedCIDRs []string `yaml:"allowed_cidrs"`
	MaxTools     int      `yaml:"max_tools"`
	RateLimitRPM int      `yaml:"rate_limit_rpm"`
}

type fileConfig struct {
	HTTP          fileHTTPConfig          `yaml:"http"`
	Database      fileDatabaseConfig      `yaml:"database"`
	OTel          fileOTelConfig          `yaml:"otel"`
	Log           fileLogConfig           `yaml:"log"`
	Auth          fileAuthConfig          `yaml:"auth"`
	ToolDiscovery fileToolDiscoveryConfig `yaml:"tool_discovery"`
	BootstrapFile string                  `yaml:"bootstrap_file"`
	InstanceTags  []InstanceTagSpec       `yaml:"instance_tags"`
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
			ShutdownDrainDelay: "5s",
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
			GroupsClaim:     "groups",
			EmailClaim:      "email",
			RolesClaim:      "realm_access.roles",
			AdminRole:       "admin",
			ReviewerGroup:   "registry-reviewers",
			LocalLogin:      true,
			AccessTokenTTL:  "15m",
			RefreshTokenTTL: "12h",
			SweepInterval:   "15m",
		},
		ToolDiscovery: fileToolDiscoveryConfig{
			Enabled:      true,
			Timeout:      "10s",
			MaxTools:     500,
			RateLimitRPM: 30,
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
// defaults. Any value that does not parse or is out of range is an error;
// all such problems are reported together.
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

	var p parser

	readTimeout := p.duration("http.read_timeout", fc.HTTP.ReadTimeout, 30*time.Second)
	writeTimeout := p.duration("http.write_timeout", fc.HTTP.WriteTimeout, 30*time.Second)
	idleTimeout := p.duration("http.idle_timeout", fc.HTTP.IdleTimeout, 120*time.Second)
	shutdownDrainDelay := p.duration("http.shutdown_drain_delay", fc.HTTP.ShutdownDrainDelay, 5*time.Second)
	accessTokenTTL := p.duration("auth.access_token_ttl", fc.Auth.AccessTokenTTL, 15*time.Minute)
	refreshTokenTTL := p.duration("auth.refresh_token_ttl", fc.Auth.RefreshTokenTTL, 12*time.Hour)
	sweepInterval := p.duration("auth.sweep_interval", fc.Auth.SweepInterval, 15*time.Minute)
	discoveryTimeout := p.duration("tool_discovery.timeout", fc.ToolDiscovery.Timeout, 10*time.Second)

	maxConns := p.envInt("DATABASE_MAX_CONNS", fc.Database.MaxConns)
	minConns := p.envInt("DATABASE_MIN_CONNS", fc.Database.MinConns)
	switch {
	case maxConns < 1 || maxConns > math.MaxInt32:
		p.errorf("DATABASE_MAX_CONNS (database.max_conns) must be between 1 and %d, got %d", math.MaxInt32, maxConns)
		maxConns, minConns = 1, 0
	case minConns < 0 || minConns > maxConns:
		p.errorf("DATABASE_MIN_CONNS (database.min_conns) must be between 0 and DATABASE_MAX_CONNS (%d), got %d", maxConns, minConns)
		minConns = 0
	}

	// Build final config: env vars win over file values.
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr:               envString("HTTP_ADDR", fc.HTTP.Addr),
			ReadTimeout:        p.envDuration("HTTP_READ_TIMEOUT", readTimeout),
			WriteTimeout:       p.envDuration("HTTP_WRITE_TIMEOUT", writeTimeout),
			IdleTimeout:        p.envDuration("HTTP_IDLE_TIMEOUT", idleTimeout),
			CORSOrigins:        envStringSlice("CORS_ALLOWED_ORIGINS", fc.HTTP.CORSOrigins),
			TrustedProxyCIDR:   envString("TRUSTED_PROXY_CIDR", fc.HTTP.TrustedProxyCIDR),
			PublicRateLimitRPM: p.envInt("PUBLIC_RATE_LIMIT_RPM", fc.HTTP.PublicRateLimitRPM),
			PublicBaseURL:      envString("PUBLIC_BASE_URL", fc.HTTP.PublicBaseURL),
			ShutdownDrainDelay: p.envDuration("SHUTDOWN_DRAIN_DELAY", shutdownDrainDelay),
		},
		Database: DatabaseConfig{
			URL:      envString("DATABASE_URL", fc.Database.URL),
			MaxConns: int32(maxConns), //nolint:gosec // range-checked above
			MinConns: int32(minConns), //nolint:gosec // range-checked above
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
			EmailClaim:             envString("AUTH_EMAIL_CLAIM", fc.Auth.EmailClaim),
			RolesClaim:             envString("OIDC_ROLES_CLAIM", fc.Auth.RolesClaim),
			AdminRole:              envString("OIDC_ADMIN_ROLE", fc.Auth.AdminRole),
			OIDCAudience:           envString("OIDC_AUDIENCE", fc.Auth.OIDCAudience),
			ReviewerGroup:          envString("AUTH_REVIEWER_GROUP", fc.Auth.ReviewerGroup),
			LocalLoginEnabled:      p.envBool("AUTH_LOCAL_LOGIN_ENABLED", fc.Auth.LocalLogin),
			BootstrapAdminEmail:    envString("AUTH_BOOTSTRAP_ADMIN_EMAIL", fc.Auth.BootstrapAdminEmail),
			BootstrapAdminPassword: envString("AUTH_BOOTSTRAP_ADMIN_PASSWORD", ""),
			JWTSigningKey:          envString("JWT_SIGNING_KEY", ""),
			JWTSigningSeed:         envString("JWT_SIGNING_SEED", ""),
			AccessTokenTTL:         p.envDuration("ACCESS_TOKEN_TTL", accessTokenTTL),
			RefreshTokenTTL:        p.envDuration("REFRESH_TOKEN_TTL", refreshTokenTTL),
			SweepInterval:          p.envDuration("AUTH_SWEEP_INTERVAL", sweepInterval),
		},
		ToolDiscovery: ToolDiscoveryConfig{
			Enabled:      p.envBool("TOOL_DISCOVERY_ENABLED", fc.ToolDiscovery.Enabled),
			Timeout:      p.envDuration("TOOL_DISCOVERY_TIMEOUT", discoveryTimeout),
			AllowedCIDRs: p.prefixes("TOOL_DISCOVERY_ALLOWED_CIDRS (tool_discovery.allowed_cidrs)", envStringSlice("TOOL_DISCOVERY_ALLOWED_CIDRS", fc.ToolDiscovery.AllowedCIDRs)),
			MaxTools:     p.envInt("TOOL_DISCOVERY_MAX_TOOLS", fc.ToolDiscovery.MaxTools),
			RateLimitRPM: p.envInt("TOOL_DISCOVERY_RATE_LIMIT_RPM", fc.ToolDiscovery.RateLimitRPM),
		},
		BootstrapFile: envString("BOOTSTRAP_FILE", fc.BootstrapFile),
		InstanceTags:  p.envInstanceTags("INSTANCE_TAGS", fc.InstanceTags),
	}

	cfg.validate(&p)
	if len(p.errs) > 0 {
		return nil, fmt.Errorf("invalid configuration: %w", errors.Join(p.errs...))
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

func (c *Config) validate(p *parser) {
	if c.Database.URL == "" {
		p.errorf("DATABASE_URL is required")
	}

	// OIDC is brokered only when a confidential client is fully configured
	// (both ID and secret). Mirror cmd/server's enablement check so validation
	// and runtime agree on when OIDC is "on". A half-configured client is almost
	// always a mistake that would otherwise silently leave OIDC disabled.
	hasClientID := c.Auth.OIDCClientID != ""
	hasClientSecret := c.Auth.OIDCClientSecret != ""
	if hasClientID != hasClientSecret {
		p.errorf("OIDC is half-configured: set both OIDC_CLIENT_ID and OIDC_CLIENT_SECRET to enable OIDC, or neither to disable it")
	}
	oidcEnabled := hasClientID && hasClientSecret

	// OIDC_ISSUER is only needed when OIDC is enabled — the broker validates the
	// id_token `iss` against it. Local-login-only deployments run without an IdP
	// (decision N), so an empty issuer is valid when OIDC is off.
	if oidcEnabled && c.Auth.OIDCIssuer == "" {
		p.errorf("OIDC_ISSUER is required when OIDC is enabled (OIDC_CLIENT_ID and OIDC_CLIENT_SECRET are set)")
	}

	// At least one front door must be open, or no one can log in.
	if !oidcEnabled && !c.Auth.LocalLoginEnabled {
		p.errorf("no login method enabled: set AUTH_LOCAL_LOGIN_ENABLED=true, or configure OIDC (OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET)")
	}

	if c.Auth.BootstrapAdminEmail != "" && c.Auth.BootstrapAdminPassword == "" {
		p.errorf("AUTH_BOOTSTRAP_ADMIN_PASSWORD is required when AUTH_BOOTSTRAP_ADMIN_EMAIL is set")
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		p.errorf("LOG_LEVEL (log.level) must be one of debug, info, warn, error, got %q", c.Log.Level)
	}

	if c.ToolDiscovery.Timeout <= 0 {
		p.errorf("TOOL_DISCOVERY_TIMEOUT (tool_discovery.timeout) must be a positive duration, got %s", c.ToolDiscovery.Timeout)
	}
	if c.ToolDiscovery.MaxTools <= 0 {
		p.errorf("TOOL_DISCOVERY_MAX_TOOLS (tool_discovery.max_tools) must be positive, got %d", c.ToolDiscovery.MaxTools)
	}
	if c.ToolDiscovery.RateLimitRPM <= 0 {
		p.errorf("TOOL_DISCOVERY_RATE_LIMIT_RPM (tool_discovery.rate_limit_rpm) must be positive, got %d", c.ToolDiscovery.RateLimitRPM)
	}

	if c.HTTP.PublicRateLimitRPM <= 0 {
		p.errorf("PUBLIC_RATE_LIMIT_RPM (http.public_rate_limit_rpm) must be positive, got %d", c.HTTP.PublicRateLimitRPM)
	}

	// Zero means "no timeout" (or no drain); only a negative value is meaningless.
	for _, t := range []struct {
		name string
		d    time.Duration
	}{
		{"HTTP_READ_TIMEOUT (http.read_timeout)", c.HTTP.ReadTimeout},
		{"HTTP_WRITE_TIMEOUT (http.write_timeout)", c.HTTP.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT (http.idle_timeout)", c.HTTP.IdleTimeout},
		{"SHUTDOWN_DRAIN_DELAY (http.shutdown_drain_delay)", c.HTTP.ShutdownDrainDelay},
	} {
		if t.d < 0 {
			p.errorf("%s must not be negative, got %s", t.name, t.d)
		}
	}

	for _, t := range []struct {
		name string
		d    time.Duration
	}{
		{"ACCESS_TOKEN_TTL (auth.access_token_ttl)", c.Auth.AccessTokenTTL},
		{"REFRESH_TOKEN_TTL (auth.refresh_token_ttl)", c.Auth.RefreshTokenTTL},
		{"AUTH_SWEEP_INTERVAL (auth.sweep_interval)", c.Auth.SweepInterval},
	} {
		if t.d <= 0 {
			p.errorf("%s must be a positive duration, got %s", t.name, t.d)
		}
	}

	// Accepting IdP service-account tokens requires the broker (its JWKS + issuer
	// verify those tokens). An audience with no broker is a no-op misconfig.
	if c.Auth.OIDCAudience != "" && !oidcEnabled {
		p.errorf("OIDC_AUDIENCE requires OIDC to be enabled (set OIDC_CLIENT_ID and OIDC_CLIENT_SECRET)")
	}

	// Config-managed instance tags fail fast at boot — a typo'd color or a
	// duplicate slug must not surface as a half-reconciled vocabulary.
	seenTags := make(map[string]struct{}, len(c.InstanceTags))
	for i, t := range c.InstanceTags {
		if t.Slug == "" || t.Name == "" {
			p.errorf("instance_tags[%d]: slug and name are required", i)
			continue
		}
		if err := domain.ValidateSlug(t.Slug); err != nil {
			p.errorf("instance_tags[%d]: %w", i, err)
		}
		if t.Color != "" {
			if err := domain.ValidateTagColor(t.Color); err != nil {
				p.errorf("instance_tags[%d]: %w", i, err)
			}
		}
		if _, dup := seenTags[t.Slug]; dup {
			p.errorf("instance_tags[%d]: duplicate slug %q", i, t.Slug)
		}
		seenTags[t.Slug] = struct{}{}
	}
}

// ── env helpers ───────────────────────────────────────────────────────────────

// parser collects every invalid setting so Load reports them all at once.
// Only non-secret settings go through it: an error echoes the offending value.
type parser struct {
	errs []error
}

func (p *parser) errorf(format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf(format, args...))
}

// envInstanceTags parses the INSTANCE_TAGS env var as a JSON array of tag
// specs. A structured list cannot follow the comma-separated convention of
// the other slice env vars, so JSON is the env-side encoding.
func (p *parser) envInstanceTags(key string, def []InstanceTagSpec) []InstanceTagSpec {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var tags []InstanceTagSpec
	if err := json.Unmarshal([]byte(v), &tags); err != nil {
		p.errorf("%s must be a JSON array of {slug, name, description, color, active}: %w", key, err)
		return def
	}
	return tags
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (p *parser) envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.errorf("%s: %q is not an integer", key, v)
		return def
	}
	return n
}

// envBool accepts the forms strconv.ParseBool understands
// (1/t/T/TRUE/true/0/f/F/FALSE/false, …).
func (p *parser) envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.errorf("%s: %q is not a boolean (use true or false)", key, v)
		return def
	}
	return b
}

func (p *parser) envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return p.duration(key, v, def)
}

func (p *parser) duration(key, s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		p.errorf("%s: %q is not a duration (use a unit, e.g. \"30s\", \"15m\", \"12h\")", key, s)
		return def
	}
	return d
}

func (p *parser) prefixes(key string, raw []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(raw))
	for _, r := range raw {
		pfx, err := netip.ParsePrefix(strings.TrimSpace(r))
		if err != nil {
			p.errorf("%s: %q is not a CIDR (e.g. \"10.20.0.0/16\")", key, r)
			continue
		}
		out = append(out, pfx.Masked())
	}
	return out
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
