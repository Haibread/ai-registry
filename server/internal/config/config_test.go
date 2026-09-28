package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/config"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// ── existing env-var tests (now pass "" as config file) ─────────────────────

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"HTTP addr", cfg.HTTP.Addr, ":8081"},
		{"OTel service name", cfg.OTel.ServiceName, "ai-registry-server"},
		{"OTel service version", cfg.OTel.ServiceVersion, "0.1.0"},
		{"log level", cfg.Log.Level, "info"},
		{"db max conns", cfg.Database.MaxConns, int32(25)},
		{"db min conns", cfg.Database.MinConns, int32(5)},
		{"public rate limit RPM default", cfg.HTTP.PublicRateLimitRPM, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("OTEL_SERVICE_NAME", "my-service")
	t.Setenv("DATABASE_MAX_CONNS", "50")
	t.Setenv("PUBLIC_RATE_LIMIT_RPM", "5000")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"HTTP addr override", cfg.HTTP.Addr, ":9090"},
		{"log level override", cfg.Log.Level, "debug"},
		{"OTel service name override", cfg.OTel.ServiceName, "my-service"},
		{"db max conns override", cfg.Database.MaxConns, int32(50)},
		{"public rate limit RPM override", cfg.HTTP.PublicRateLimitRPM, 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")

	_, err := config.Load("")
	if err == nil {
		t.Error("expected error when DATABASE_URL is empty, got nil")
	}
}

// OIDC_ISSUER is only required when OIDC is actually enabled (both client ID
// and secret set). With OIDC configured but no issuer, Load must fail.
func TestLoad_MissingOIDCIssuer_WhenOIDCEnabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_ID", "ai-registry-server")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cret")

	_, err := config.Load("")
	if err == nil {
		t.Error("expected error when OIDC is enabled but OIDC_ISSUER is empty, got nil")
	}
}

// A local-login-only deployment (no OIDC client configured) must load without
// an OIDC_ISSUER — the registry runs without an external IdP (decision N).
func TestLoad_LocalLoginOnly_NoOIDC_OK(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected local-login-only config to load, got %v", err)
	}
	if cfg.Auth.OIDCIssuer != "" {
		t.Errorf("OIDCIssuer = %q, want empty", cfg.Auth.OIDCIssuer)
	}
	if !cfg.Auth.LocalLoginEnabled {
		t.Error("LocalLoginEnabled = false, want true (default front door)")
	}
}

// With both front doors closed — local login disabled and no OIDC client — Load
// must fail rather than booting a registry no one can log in to.
func TestLoad_NoLoginMethod_Error(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "false")

	if _, err := config.Load(""); err == nil {
		t.Error("expected error when no login method is enabled, got nil")
	}
}

// A half-configured OIDC client (ID without secret, or vice versa) is rejected
// so it can't silently leave OIDC disabled.
func TestLoad_OIDCHalfConfigured_Error(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realm")
	t.Setenv("OIDC_CLIENT_ID", "ai-registry-server")
	t.Setenv("OIDC_CLIENT_SECRET", "")

	if _, err := config.Load(""); err == nil {
		t.Error("expected error when only OIDC_CLIENT_ID is set without a secret, got nil")
	}
}

// OIDC_AUDIENCE accepts IdP service-account tokens, which the broker
// verifies — so it is meaningless (and rejected) without OIDC enabled.
func TestLoad_AudienceWithoutOIDC_Error(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "true")
	t.Setenv("OIDC_AUDIENCE", "ai-registry")

	if _, err := config.Load(""); err == nil {
		t.Error("expected error when OIDC_AUDIENCE is set without OIDC enabled, got nil")
	}
}

func TestLoad_AudienceWithOIDC_OK(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realms/test")
	t.Setenv("OIDC_CLIENT_ID", "ai-registry-server")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cret")
	t.Setenv("OIDC_AUDIENCE", "ai-registry")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDCAudience != "ai-registry" {
		t.Fatalf("OIDCAudience = %q, want ai-registry", cfg.Auth.OIDCAudience)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realms/test")

	_, err := config.Load("")
	if err != nil {
		t.Errorf("expected no error for valid config, got %v", err)
	}
}

func TestLoad_CORSOrigins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, http://localhost:3001")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.HTTP.CORSOrigins) != 2 {
		t.Fatalf("CORSOrigins len = %d, want 2", len(cfg.HTTP.CORSOrigins))
	}
	if cfg.HTTP.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("CORSOrigins[0] = %q, want %q", cfg.HTTP.CORSOrigins[0], "http://localhost:3000")
	}
	if cfg.HTTP.CORSOrigins[1] != "http://localhost:3001" {
		t.Errorf("CORSOrigins[1] = %q, want %q", cfg.HTTP.CORSOrigins[1], "http://localhost:3001")
	}
}

// ── YAML file tests ──────────────────────────────────────────────────────────

// writeConfigFile creates a temp YAML config file with the given content and
// returns its path. The file is removed when t finishes.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func TestLoad_FileConfig_Basic(t *testing.T) {
	path := writeConfigFile(t, `
http:
  addr: ":9999"
  read_timeout: "60s"
  cors_origins:
    - "https://example.com"
database:
  url: "postgres://file:file@db/registry"
  max_conns: 10
  min_conns: 2
otel:
  service_name: "from-file"
log:
  level: "warn"
auth:
  oidc_issuer: "https://auth.example.com/realms/test"
`)

	// Ensure env vars do not interfere.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("OTEL_SERVICE_NAME", "")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"addr", cfg.HTTP.Addr, ":9999"},
		{"read timeout", cfg.HTTP.ReadTimeout, 60 * time.Second},
		{"db url", cfg.Database.URL, "postgres://file:file@db/registry"},
		{"db max conns", cfg.Database.MaxConns, int32(10)},
		{"db min conns", cfg.Database.MinConns, int32(2)},
		{"service name", cfg.OTel.ServiceName, "from-file"},
		{"log level", cfg.Log.Level, "warn"},
		{"oidc issuer", cfg.Auth.OIDCIssuer, "https://auth.example.com/realms/test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}

	if len(cfg.HTTP.CORSOrigins) != 1 || cfg.HTTP.CORSOrigins[0] != "https://example.com" {
		t.Errorf("CORSOrigins = %v, want [https://example.com]", cfg.HTTP.CORSOrigins)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://file:file@db/registry"
http:
  addr: ":7777"
log:
  level: "warn"
auth:
  oidc_issuer: "https://from-file.example.com/realm"
`)

	// Env vars should win over file values.
	t.Setenv("DATABASE_URL", "postgres://env:env@db/registry")
	t.Setenv("OIDC_ISSUER", "https://from-env.example.com/realm")
	t.Setenv("HTTP_ADDR", ":8888")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.URL != "postgres://env:env@db/registry" {
		t.Errorf("DATABASE_URL: got %q, want env value", cfg.Database.URL)
	}
	if cfg.Auth.OIDCIssuer != "https://from-env.example.com/realm" {
		t.Errorf("OIDC_ISSUER: got %q, want env value", cfg.Auth.OIDCIssuer)
	}
	if cfg.HTTP.Addr != ":8888" {
		t.Errorf("HTTP_ADDR: got %q, want env value", cfg.HTTP.Addr)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("LOG_LEVEL: got %q, want env value", cfg.Log.Level)
	}
}

func TestLoad_FileDefaults_PartialFile(t *testing.T) {
	// A file that only sets the required fields; everything else falls back to
	// built-in defaults.
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
`)

	// Clear env vars so only file + defaults apply.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Built-in defaults should survive for keys not in the file.
	if cfg.HTTP.Addr != ":8081" {
		t.Errorf("HTTP addr: got %q, want default :8081", cfg.HTTP.Addr)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log level: got %q, want default info", cfg.Log.Level)
	}
	if cfg.Database.MaxConns != 25 {
		t.Errorf("db max conns: got %d, want default 25", cfg.Database.MaxConns)
	}
}

func TestLoad_MissingFile_NotAnError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realm")

	// Point at a file that does not exist — this must not error.
	_, err := config.Load("/tmp/this-file-does-not-exist-ai-registry.yaml")
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeConfigFile(t, `
http:
  addr: [this is: not: valid yaml
`)

	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realm")

	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoad_UnknownKey_ReturnsError(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
unknown_section:
  foo: bar
`)

	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")

	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for unknown YAML key, got nil")
	}
}

func TestLoad_CONFIGFILEEnvVar(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://via:env-var@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
`)

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")

	cfg, err := config.Load("") // empty string → falls back to CONFIG_FILE
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.URL != "postgres://via:env-var@localhost/db" {
		t.Errorf("DATABASE_URL: got %q, want value from CONFIG_FILE", cfg.Database.URL)
	}
}

// ── PUBLIC_BASE_URL & BOOTSTRAP_FILE — env+YAML+default rule ─────────────────
//
// Every config knob is reachable via env var, YAML key, AND a default; these
// tests pin that rule for PUBLIC_BASE_URL and BOOTSTRAP_FILE.

func TestLoad_PublicBaseURL_DefaultEmpty(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("PUBLIC_BASE_URL", "")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Default is empty so misconfigured deployments fail loudly in the
	// well-known handlers rather than advertising a fake localhost URL.
	if cfg.HTTP.PublicBaseURL != "" {
		t.Errorf("PublicBaseURL default = %q, want empty", cfg.HTTP.PublicBaseURL)
	}
}

func TestLoad_PublicBaseURL_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("PUBLIC_BASE_URL", "https://registry.example.com")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTP.PublicBaseURL != "https://registry.example.com" {
		t.Errorf("PublicBaseURL = %q, want https://registry.example.com", cfg.HTTP.PublicBaseURL)
	}
}

func TestLoad_PublicBaseURL_FromYAML(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
http:
  public_base_url: "https://from-file.example.com"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("PUBLIC_BASE_URL", "")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTP.PublicBaseURL != "https://from-file.example.com" {
		t.Errorf("PublicBaseURL = %q, want https://from-file.example.com", cfg.HTTP.PublicBaseURL)
	}
}

func TestLoad_PublicBaseURL_EnvWinsOverYAML(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
http:
  public_base_url: "https://from-file.example.com"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("PUBLIC_BASE_URL", "https://from-env.example.com")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTP.PublicBaseURL != "https://from-env.example.com" {
		t.Errorf("PublicBaseURL = %q, want env value to win over YAML", cfg.HTTP.PublicBaseURL)
	}
}

func TestLoad_BootstrapFile_DefaultEmpty(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("BOOTSTRAP_FILE", "")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BootstrapFile != "" {
		t.Errorf("BootstrapFile default = %q, want empty", cfg.BootstrapFile)
	}
}

func TestLoad_BootstrapFile_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("BOOTSTRAP_FILE", "/etc/ai-registry/bootstrap.yaml")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BootstrapFile != "/etc/ai-registry/bootstrap.yaml" {
		t.Errorf("BootstrapFile = %q, want /etc/ai-registry/bootstrap.yaml", cfg.BootstrapFile)
	}
}

func TestLoad_BootstrapFile_FromYAML(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
bootstrap_file: "/srv/data/bootstrap.yaml"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("BOOTSTRAP_FILE", "")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BootstrapFile != "/srv/data/bootstrap.yaml" {
		t.Errorf("BootstrapFile = %q, want /srv/data/bootstrap.yaml", cfg.BootstrapFile)
	}
}

func TestLoad_BootstrapFile_EnvWinsOverYAML(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
bootstrap_file: "/from/file.yaml"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("BOOTSTRAP_FILE", "/from/env.yaml")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BootstrapFile != "/from/env.yaml" {
		t.Errorf("BootstrapFile = %q, want env value to win over YAML", cfg.BootstrapFile)
	}
}

// ── Local-auth knobs — env+YAML+default rule ─────────────────────

func TestLoad_LocalAuth_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Auth.LocalLoginEnabled {
		t.Error("LocalLoginEnabled default = false, want true")
	}
	if cfg.Auth.BootstrapAdminEmail != "" {
		t.Errorf("BootstrapAdminEmail default = %q, want empty", cfg.Auth.BootstrapAdminEmail)
	}
}

func TestLoad_LocalAuth_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	// Local login is disabled here, so OIDC must be the open front door —
	// otherwise validation rejects a config with no login method.
	t.Setenv("OIDC_CLIENT_ID", "ai-registry-server")
	t.Setenv("OIDC_CLIENT_SECRET", "broker-secret")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "false")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD", "s3cret")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.LocalLoginEnabled {
		t.Error("AUTH_LOCAL_LOGIN_ENABLED=false should disable local login")
	}
	if cfg.Auth.BootstrapAdminEmail != "admin@example.com" || cfg.Auth.BootstrapAdminPassword != "s3cret" {
		t.Errorf("bootstrap admin = (%q,%q), want env values", cfg.Auth.BootstrapAdminEmail, cfg.Auth.BootstrapAdminPassword)
	}
}

func TestLoad_LocalAuth_FromYAML(t *testing.T) {
	path := writeConfigFile(t, `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
  oidc_client_id: "ai-registry-server"
  local_login: false
  bootstrap_admin_email: "boot@example.com"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_EMAIL", "")
	// Local login is off in the file, so OIDC must be the open front door.
	// The client secret is a credential supplied via env (never the file).
	t.Setenv("OIDC_CLIENT_SECRET", "broker-secret")
	// The bootstrap email comes from the YAML file; its password is a
	// credential supplied via env (never the file), so set it to satisfy the
	// password-required-with-email validation.
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD", "from-secret")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.LocalLoginEnabled {
		t.Error("local_login: false in YAML should disable local login")
	}
	if cfg.Auth.BootstrapAdminEmail != "boot@example.com" {
		t.Errorf("BootstrapAdminEmail = %q, want YAML value", cfg.Auth.BootstrapAdminEmail)
	}
}

// TestLoad_BootstrapAdmin_PasswordRequiredWithEmail pins the validation that a
// bootstrap admin email without a password is rejected (the password is a
// credential that cannot live in the config file).
func TestLoad_BootstrapAdmin_PasswordRequiredWithEmail(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD", "")

	if _, err := config.Load(""); err == nil {
		t.Error("expected error when bootstrap admin email is set without a password")
	}
}

func TestLoad_ExplicitPathWinsOverCONFIGFILE(t *testing.T) {
	pathA := writeConfigFile(t, `
database:
  url: "postgres://path-a:x@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
`)
	pathB := writeConfigFile(t, `
database:
  url: "postgres://path-b:x@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
`)

	t.Setenv("CONFIG_FILE", pathB) // should be ignored when explicit path given
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")

	cfg, err := config.Load(pathA) // explicit path wins
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.URL != "postgres://path-a:x@localhost/db" {
		t.Errorf("DATABASE_URL: got %q, want path-a value", cfg.Database.URL)
	}
}

// ── instance tags ─────────────────────────────────────────────────────────────

func TestLoad_InstanceTags_FromFile(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	path := writeConfigFile(t, `
instance_tags:
  - slug: free
    name: Free
    description: No cost to use
    color: green
  - slug: legacy
    name: Legacy
    active: false
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.InstanceTags) != 2 {
		t.Fatalf("got %d instance tags, want 2", len(cfg.InstanceTags))
	}
	free := cfg.InstanceTags[0]
	if free.Slug != "free" || free.Name != "Free" || free.Color != "green" || free.Active != nil {
		t.Errorf("unexpected first tag: %+v", free)
	}
	legacy := cfg.InstanceTags[1]
	if legacy.Slug != "legacy" || legacy.Active == nil || *legacy.Active {
		t.Errorf("unexpected second tag: %+v", legacy)
	}
}

func TestLoad_InstanceTags_EnvJSONWinsOverFile(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("INSTANCE_TAGS", `[{"slug":"early-access","name":"Early Access","color":"yellow","active":false}]`)
	path := writeConfigFile(t, `
instance_tags:
  - slug: free
    name: Free
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.InstanceTags) != 1 || cfg.InstanceTags[0].Slug != "early-access" {
		t.Fatalf("env must win over file, got %+v", cfg.InstanceTags)
	}
	if a := cfg.InstanceTags[0].Active; a == nil || *a {
		t.Errorf("active=false from env JSON not honoured: %+v", cfg.InstanceTags[0])
	}
}

func TestLoad_InstanceTags_Validation(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"malformed JSON", `not-json`},
		{"missing name", `[{"slug":"x"}]`},
		{"missing slug", `[{"name":"X"}]`},
		{"bad slug", `[{"slug":"Not A Slug","name":"X"}]`},
		{"bad color", `[{"slug":"x","name":"X","color":"magenta"}]`},
		{"duplicate slug", `[{"slug":"x","name":"X"},{"slug":"x","name":"Y"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
			t.Setenv("INSTANCE_TAGS", tt.env)
			if _, err := config.Load(""); err == nil {
				t.Errorf("Load() with INSTANCE_TAGS=%q should fail", tt.env)
			}
		})
	}
}

func TestLoad_AuthSweepInterval(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		env     string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: 15 * time.Minute},
		{name: "file", file: "auth:\n  sweep_interval: \"5m\"\n", want: 5 * time.Minute},
		{name: "env beats file", file: "auth:\n  sweep_interval: \"5m\"\n", env: "30s", want: 30 * time.Second},
		{name: "zero rejected", env: "0s", wantErr: true},
		{name: "negative rejected", env: "-1m", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
			t.Setenv("CONFIG_FILE", "")
			t.Setenv("AUTH_SWEEP_INTERVAL", tt.env)
			path := ""
			if tt.file != "" {
				path = writeConfigFile(t, tt.file)
			}
			cfg, err := config.Load(path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = nil error, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Auth.SweepInterval != tt.want {
				t.Errorf("SweepInterval = %v, want %v", cfg.Auth.SweepInterval, tt.want)
			}
		})
	}
}

func TestLoad_InvalidEnvValue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  []string
	}{
		{"int not a number", "DATABASE_MAX_CONNS", "abc", []string{"DATABASE_MAX_CONNS", `"abc"`}},
		{"int with unit", "PUBLIC_RATE_LIMIT_RPM", "100/m", []string{"PUBLIC_RATE_LIMIT_RPM", `"100/m"`}},
		{"bool yes", "AUTH_LOCAL_LOGIN_ENABLED", "yes", []string{"AUTH_LOCAL_LOGIN_ENABLED", `"yes"`}},
		{"duration without unit", "ACCESS_TOKEN_TTL", "15", []string{"ACCESS_TOKEN_TTL", `"15"`}},
		{"duration garbage", "HTTP_READ_TIMEOUT", "soon", []string{"HTTP_READ_TIMEOUT", `"soon"`}},
		{"instance tags not JSON", "INSTANCE_TAGS", "not-json", []string{"INSTANCE_TAGS"}},
		{"negative timeout", "HTTP_IDLE_TIMEOUT", "-1s", []string{"HTTP_IDLE_TIMEOUT", "negative"}},
		{"drain delay without unit", "SHUTDOWN_DRAIN_DELAY", "5", []string{"SHUTDOWN_DRAIN_DELAY", `"5"`}},
		{"zero access TTL", "ACCESS_TOKEN_TTL", "0s", []string{"ACCESS_TOKEN_TTL", "positive"}},
		{"negative refresh TTL", "REFRESH_TOKEN_TTL", "-1h", []string{"REFRESH_TOKEN_TTL", "positive"}},
		{"zero rate limit", "PUBLIC_RATE_LIMIT_RPM", "0", []string{"PUBLIC_RATE_LIMIT_RPM", "positive"}},
		{"negative rate limit", "PUBLIC_RATE_LIMIT_RPM", "-5", []string{"PUBLIC_RATE_LIMIT_RPM", "positive"}},
		{"zero max conns", "DATABASE_MAX_CONNS", "0", []string{"DATABASE_MAX_CONNS"}},
		{"max conns overflows int32", "DATABASE_MAX_CONNS", "4294967296", []string{"DATABASE_MAX_CONNS"}},
		{"min conns above max", "DATABASE_MIN_CONNS", "30", []string{"DATABASE_MIN_CONNS"}},
		{"negative min conns", "DATABASE_MIN_CONNS", "-1", []string{"DATABASE_MIN_CONNS"}},
		{"unknown log level", "LOG_LEVEL", "DEBUG", []string{"LOG_LEVEL", `"DEBUG"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
			t.Setenv("CONFIG_FILE", "")
			t.Setenv(tt.key, tt.value)
			_, err := config.Load("")
			if err == nil {
				t.Fatalf("Load() with %s=%q = nil error, want an error", tt.key, tt.value)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %s", err, w)
				}
			}
		})
	}
}

func TestLoad_InvalidFileValue(t *testing.T) {
	tests := []struct {
		name string
		file string
		want []string
	}{
		{"duration without unit", "http:\n  read_timeout: \"30\"\n", []string{"http.read_timeout", `"30"`}},
		{"ttl garbage", "auth:\n  refresh_token_ttl: \"forever\"\n", []string{"auth.refresh_token_ttl", `"forever"`}},
		{"sweep interval garbage", "auth:\n  sweep_interval: \"often\"\n", []string{"auth.sweep_interval", `"often"`}},
		{"negative drain delay", "http:\n  shutdown_drain_delay: \"-1s\"\n", []string{"http.shutdown_drain_delay", "negative"}},
		{"negative ttl", "auth:\n  access_token_ttl: \"-5m\"\n", []string{"auth.access_token_ttl", "positive"}},
		{"min above max", "database:\n  max_conns: 2\n  min_conns: 3\n", []string{"database.min_conns"}},
		{"zero rate limit", "http:\n  public_rate_limit_rpm: 0\n", []string{"http.public_rate_limit_rpm"}},
		{"unknown log level", "log:\n  level: \"verbose\"\n", []string{"log.level", `"verbose"`}},
		{"int as string", "database:\n  max_conns: \"many\"\n", []string{"line 2", "many"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
			t.Setenv("CONFIG_FILE", "")
			_, err := config.Load(writeConfigFile(t, tt.file))
			if err == nil {
				t.Fatalf("Load() = nil error, want an error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %s", err, w)
				}
			}
		})
	}
}

func TestLoad_ValidParsedValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("CONFIG_FILE", "")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "1")
	t.Setenv("HTTP_READ_TIMEOUT", "0s")
	t.Setenv("ACCESS_TOKEN_TTL", "90s")
	t.Setenv("DATABASE_MAX_CONNS", "10")
	t.Setenv("DATABASE_MIN_CONNS", "10")
	t.Setenv("LOG_LEVEL", "warn")
	path := writeConfigFile(t, "auth:\n  refresh_token_ttl: \"24h\"\nhttp:\n  idle_timeout: \"1m30s\"\n")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"bool", cfg.Auth.LocalLoginEnabled, true},
		{"zero timeout means none", cfg.HTTP.ReadTimeout, time.Duration(0)},
		{"env duration", cfg.Auth.AccessTokenTTL, 90 * time.Second},
		{"file duration", cfg.Auth.RefreshTokenTTL, 24 * time.Hour},
		{"compound file duration", cfg.HTTP.IdleTimeout, 90 * time.Second},
		{"max conns", cfg.Database.MaxConns, int32(10)},
		{"min conns equal to max", cfg.Database.MinConns, int32(10)},
		{"log level", cfg.Log.Level, "warn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_ReportsEveryInvalidValue(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_MAX_CONNS", "abc")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "yes")
	t.Setenv("ACCESS_TOKEN_TTL", "15")
	path := writeConfigFile(t, "http:\n  write_timeout: \"30\"\n")

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load() = nil error, want an error")
	}
	for _, w := range []string{"DATABASE_URL", "DATABASE_MAX_CONNS", "AUTH_LOCAL_LOGIN_ENABLED", "ACCESS_TOKEN_TTL", "http.write_timeout"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not mention %s", err, w)
		}
	}
}

func TestLoad_ErrorNeverEchoesSecrets(t *testing.T) {
	secrets := map[string]string{
		"DATABASE_URL":                  "postgres://u:dsn-secret-marker@db/x",
		"OIDC_CLIENT_SECRET":            "client-secret-marker",
		"AUTH_BOOTSTRAP_ADMIN_PASSWORD": "password-marker",
		"JWT_SIGNING_KEY":               "signing-key-marker",
		"JWT_SIGNING_SEED":              "signing-seed-marker",
	}
	for k, v := range secrets {
		t.Setenv(k, v)
	}
	t.Setenv("CONFIG_FILE", "")
	// Half-configured OIDC and an audience without a broker both name secret
	// settings; the bad values make sure every other error path runs too.
	t.Setenv("OIDC_AUDIENCE", "registry")
	t.Setenv("DATABASE_MAX_CONNS", "abc")
	t.Setenv("ACCESS_TOKEN_TTL", "15")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "no-thanks")
	t.Setenv("INSTANCE_TAGS", "{")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("Load() = nil error, want an error")
	}
	for k, v := range secrets {
		if strings.Contains(err.Error(), v) || strings.Contains(err.Error(), "marker") {
			t.Errorf("error leaks the value of %s: %q", k, err)
		}
	}
}

func TestLoad_ShutdownDrainDelay(t *testing.T) {
	yamlOnly := `
database:
  url: "postgres://p:p@localhost/db"
auth:
  oidc_issuer: "https://auth.example.com/realm"
http:
  shutdown_drain_delay: "12s"
`
	tests := []struct {
		name string
		yaml string
		env  string
		want time.Duration
	}{
		{"default", "", "", 5 * time.Second},
		{"from YAML", yamlOnly, "", 12 * time.Second},
		{"env wins over YAML", yamlOnly, "3s", 3 * time.Second},
		{"zero disables", "", "0s", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := ""
			if tt.yaml != "" {
				path = writeConfigFile(t, tt.yaml)
			}
			t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
			t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
			t.Setenv("SHUTDOWN_DRAIN_DELAY", tt.env)

			cfg, err := config.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.HTTP.ShutdownDrainDelay != tt.want {
				t.Errorf("ShutdownDrainDelay = %v, want %v", cfg.HTTP.ShutdownDrainDelay, tt.want)
			}
		})
	}
}
