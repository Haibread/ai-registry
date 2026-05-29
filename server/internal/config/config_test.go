package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/config"
)

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

func TestLoad_MissingOIDCIssuer(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")

	_, err := config.Load("")
	if err == nil {
		t.Error("expected error when OIDC_ISSUER is empty, got nil")
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
// CLAUDE.md requires every config knob to be reachable via env var, YAML key,
// AND a default. Both PUBLIC_BASE_URL and BOOTSTRAP_FILE used to bypass the
// config layer (read directly via os.Getenv from handlers and main); these
// tests pin the three-place rule for both.

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

// ── Local-auth knobs (ADR 0006) — env+YAML+default rule ─────────────────────

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
	if cfg.Auth.LocalTokenTTL != time.Hour {
		t.Errorf("LocalTokenTTL default = %v, want 1h", cfg.Auth.LocalTokenTTL)
	}
	if cfg.Auth.BootstrapAdminEmail != "" {
		t.Errorf("BootstrapAdminEmail default = %q, want empty", cfg.Auth.BootstrapAdminEmail)
	}
}

func TestLoad_LocalAuth_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "false")
	t.Setenv("AUTH_LOCAL_SIGNING_KEY", "pem-key-value")
	t.Setenv("AUTH_LOCAL_TOKEN_TTL", "30m")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD", "s3cret")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.LocalLoginEnabled {
		t.Error("AUTH_LOCAL_LOGIN_ENABLED=false should disable local login")
	}
	if cfg.Auth.LocalSigningKey != "pem-key-value" {
		t.Errorf("LocalSigningKey = %q, want env value", cfg.Auth.LocalSigningKey)
	}
	if cfg.Auth.LocalTokenTTL != 30*time.Minute {
		t.Errorf("LocalTokenTTL = %v, want 30m", cfg.Auth.LocalTokenTTL)
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
  local_login: false
  local_token_ttl: "2h"
  bootstrap_admin_email: "boot@example.com"
`)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("AUTH_LOCAL_LOGIN_ENABLED", "")
	t.Setenv("AUTH_LOCAL_TOKEN_TTL", "")
	t.Setenv("AUTH_BOOTSTRAP_ADMIN_EMAIL", "")
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
	if cfg.Auth.LocalTokenTTL != 2*time.Hour {
		t.Errorf("LocalTokenTTL = %v, want 2h from YAML", cfg.Auth.LocalTokenTTL)
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
