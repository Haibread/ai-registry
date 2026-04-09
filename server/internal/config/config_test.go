package config_test

import (
	"testing"

	"github.com/haibread/ai-registry/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")

	cfg, err := config.Load()
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

	cfg, err := config.Load()
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

	_, err := config.Load()
	if err == nil {
		t.Error("expected error when DATABASE_URL is empty, got nil")
	}
}

func TestLoad_MissingOIDCIssuer(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error when OIDC_ISSUER is empty, got nil")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "https://auth.example.com/realms/test")

	_, err := config.Load()
	if err != nil {
		t.Errorf("expected no error for valid config, got %v", err)
	}
}

func TestLoad_CORSOrigins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("OIDC_ISSUER", "http://keycloak:8080/realms/ai-registry")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, http://localhost:3001")

	cfg, err := config.Load()
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
