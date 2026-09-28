package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
)

func TestValidateHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: ""},
		{name: "https", input: "https://example.com/repo"},
		{name: "http with port", input: "http://localhost:8080/a2a"},
		{name: "uppercase scheme", input: "HTTPS://example.com"},
		{name: "at max length", input: "https://example.com/" + strings.Repeat("a", domain.MaxURLLength-20)},
		{name: "javascript", input: "javascript:alert(1)", wantErr: true},
		{name: "javascript with slashes", input: "javascript://example.com/%0aalert(1)", wantErr: true},
		{name: "data", input: "data:text/html,<script>alert(1)</script>", wantErr: true},
		{name: "relative", input: "/docs", wantErr: true},
		{name: "scheme-relative", input: "//example.com/docs", wantErr: true},
		{name: "missing host", input: "https:///path", wantErr: true},
		{name: "port only", input: "http://:8080", wantErr: true},
		{name: "opaque http", input: "http:example.com", wantErr: true},
		{name: "ftp", input: "ftp://example.com/file", wantErr: true},
		{name: "too long", input: "https://example.com/" + strings.Repeat("a", domain.MaxURLLength), wantErr: true},
		{name: "unparseable", input: "https://exa mple.com:port", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateHTTPURL("homepage_url", tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHTTPURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil && !strings.HasPrefix(err.Error(), "homepage_url ") {
				t.Errorf("error %q does not name the field", err)
			}
		})
	}
}

func TestValidateAgentVersionURLs(t *testing.T) {
	const ok = "https://example.com"
	tests := []struct {
		name                string
		endpoint, doc, icon string
		provider            string
		wantField           string
	}{
		{name: "all valid", endpoint: ok, doc: ok, icon: ok, provider: `{"organization":"Acme","url":"https://acme.example"}`},
		{name: "optional empty", endpoint: ok},
		{name: "null provider", endpoint: ok, provider: `null`},
		{name: "bad endpoint", endpoint: "javascript:alert(1)", wantField: "endpoint_url"},
		{name: "bad documentation", endpoint: ok, doc: "data:text/html,x", wantField: "documentation_url"},
		{name: "bad icon", endpoint: ok, icon: "ftp://example.com/i.png", wantField: "icon_url"},
		{name: "bad provider url", endpoint: ok, provider: `{"url":"javascript:alert(1)"}`, wantField: "provider.url"},
		{name: "provider not an object", endpoint: ok, provider: `"acme"`, wantField: "provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider json.RawMessage
			if tt.provider != "" {
				provider = json.RawMessage(tt.provider)
			}
			err := domain.ValidateAgentVersionURLs(tt.endpoint, tt.doc, tt.icon, provider)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantField+" ") {
				t.Fatalf("error = %v, want one naming %s", err, tt.wantField)
			}
		})
	}
}

func TestValidateMCPServerURLs(t *testing.T) {
	if err := domain.ValidateMCPServerURLs("", ""); err != nil {
		t.Fatalf("empty links: %v", err)
	}
	if err := domain.ValidateMCPServerURLs("https://example.com", "https://github.com/acme/srv"); err != nil {
		t.Fatalf("valid links: %v", err)
	}
	if err := domain.ValidateMCPServerURLs("https://example.com", "javascript:alert(1)"); err == nil || !strings.HasPrefix(err.Error(), "repo_url ") {
		t.Fatalf("error = %v, want one naming repo_url", err)
	}
}
