package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
)

// TestMCPServerVersion_JSONShape locks in the snake_case wire format used by
// the versions list/detail endpoints. Regression guard: before struct tags
// were added, Go's default marshaler emitted PascalCase field names, which
// meant the frontend VersionHistory read `undefined` for `published_at` and
// rendered every published version as "Draft".
func TestMCPServerVersion_JSONShape(t *testing.T) {
	published := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	v := domain.MCPServerVersion{
		ID:              "01J",
		ServerID:        "01S",
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		ProtocolVersion: "2025-03-26",
		Status:          domain.VersionStatusActive,
		PublishedAt:     &published,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"id", "server_id", "version", "runtime", "protocol_version",
		"status", "published_at", "created_at", "updated_at",
	} {
		if _, ok := out[key]; !ok {
			t.Errorf("missing JSON key %q; got %s", key, string(b))
		}
	}
	// Ensure no PascalCase leakage.
	for _, bad := range []string{"ID", "ServerID", "Version", "PublishedAt", "Status"} {
		if _, ok := out[bad]; ok {
			t.Errorf("unexpected PascalCase JSON key %q", bad)
		}
	}
}

// TestMCPServerVersion_JSONShape_Draft verifies that PublishedAt is omitted
// (not serialized as null) when the version is still a draft, so the
// frontend's `v.published_at ? ... : 'Draft'` check triggers correctly.
func TestMCPServerVersion_JSONShape_Draft(t *testing.T) {
	v := domain.MCPServerVersion{ID: "01J", Version: "0.1.0"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := out["published_at"]; ok {
		t.Errorf("published_at should be omitted for draft versions; got %s", string(b))
	}
}

func TestValidatePackages(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid npm package",
			input: `[{"registryType":"npm","identifier":"@scope/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`,
		},
		{
			name: "valid http transport",
			// "oci" is the spec-correct registryType for container images; "docker" is not allowed.
			input: `[{"registryType":"oci","identifier":"myimage","version":"1.0.0","transport":{"type":"http"}}]`,
		},
		{
			name: "valid streamable-http transport",
			input: `[{"registryType":"npm","identifier":"pkg","version":"1.0.0","transport":{"type":"streamable-http"}}]`,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "not an array",
			input:   `{"registryType":"npm"}`,
			wantErr: true,
		},
		{
			name:    "missing registryType",
			input:   `[{"identifier":"pkg","version":"1.0.0","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
		{
			name:    "missing identifier",
			input:   `[{"registryType":"npm","version":"1.0.0","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
		{
			name:    "missing version",
			input:   `[{"registryType":"npm","identifier":"pkg","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
		{
			name:    "missing transport type",
			input:   `[{"registryType":"npm","identifier":"pkg","version":"1.0.0","transport":{}}]`,
			wantErr: true,
		},
		{
			name:    "invalid transport type",
			input:   `[{"registryType":"npm","identifier":"pkg","version":"1.0.0","transport":{"type":"grpc"}}]`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "invalid registryType docker",
			input:   `[{"registryType":"docker","identifier":"img","version":"1.0.0","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
		{
			name:    "invalid registryType maven",
			input:   `[{"registryType":"maven","identifier":"com.example:pkg","version":"1.0.0","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
		{
			name:  "valid pypi registryType",
			input: `[{"registryType":"pypi","identifier":"mypackage","version":"1.0.0","transport":{"type":"stdio"}}]`,
		},
		{
			name:  "valid oci registryType",
			input: `[{"registryType":"oci","identifier":"myimage:1.0.0","version":"1.0.0","transport":{"type":"http"}}]`,
		},
		{
			name:    "version is latest",
			input:   `[{"registryType":"npm","identifier":"@t/p","version":"latest","transport":{"type":"stdio"}}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidatePackages(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePackages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateServerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "myns/myserver"},
		{name: "valid with dots", input: "my.ns/my.server"},
		{name: "valid with dashes", input: "my-ns/my-server"},
		{name: "valid with numbers", input: "ns123/srv456"},
		{name: "no slash", input: "noslash", wantErr: true},
		{name: "leading slash", input: "/leading", wantErr: true},
		{name: "trailing slash", input: "trailing/", wantErr: true},
		{name: "spaces in namespace", input: "ns with spaces/srv", wantErr: true},
		{name: "spaces in slug", input: "ns/srv with spaces", wantErr: true},
		{name: "special chars", input: "ns!/srv", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateServerName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServerName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty is ok", input: ""},
		{name: "empty object", input: `{}`},
		{name: "with tools", input: `{"tools":[{"name":"myTool"}]}`},
		{name: "invalid JSON", input: `{bad`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateCapabilities(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCapabilities() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
