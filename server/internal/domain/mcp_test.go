package domain_test

import (
	"encoding/json"
	"slices"
	"strings"
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
		ID:               "01J",
		ServerID:         "01S",
		Version:          "1.0.0",
		Runtime:          domain.RuntimeStdio,
		ProtocolVersions: []string{"2025-03-26"},
		Status:           domain.VersionStatusActive,
		PublishedAt:      &published,
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
		"id", "server_id", "version", "runtime", "protocol_versions",
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
			name:  "valid npm package",
			input: `[{"registryType":"npm","identifier":"@scope/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`,
		},
		{
			name: "valid http transport",
			// "oci" is the spec-correct registryType for container images; "docker" is not allowed.
			input: `[{"registryType":"oci","identifier":"myimage","version":"1.0.0","transport":{"type":"http"}}]`,
		},
		{
			name:  "valid streamable-http transport",
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

func TestValidateRemotes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "valid sse remote",
			input: `[{"type":"sse","url":"https://mcp.example.com/sse"}]`,
		},
		{
			name:  "valid streamable_http remote",
			input: `[{"type":"streamable_http","url":"https://mcp.example.com/mcp"}]`,
		},
		{
			name:  "valid spec-spelling streamable-http remote",
			input: `[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]`,
		},
		{
			name:  "valid http remote with plain http scheme",
			input: `[{"type":"http","url":"http://localhost:8080/mcp"}]`,
		},
		{
			name:  "multiple remotes",
			input: `[{"type":"sse","url":"https://a.example.com/sse"},{"type":"http","url":"https://b.example.com/mcp"}]`,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "not an array",
			input:   `{"type":"sse","url":"https://a.example.com"}`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "missing type",
			input:   `[{"url":"https://mcp.example.com/sse"}]`,
			wantErr: true,
		},
		{
			name:    "stdio is not a remote transport",
			input:   `[{"type":"stdio","url":"https://mcp.example.com"}]`,
			wantErr: true,
		},
		{
			name:    "invalid transport type",
			input:   `[{"type":"grpc","url":"https://mcp.example.com"}]`,
			wantErr: true,
		},
		{
			name:    "missing url",
			input:   `[{"type":"sse"}]`,
			wantErr: true,
		},
		{
			name:    "relative url",
			input:   `[{"type":"sse","url":"/sse"}]`,
			wantErr: true,
		},
		{
			name:    "non-http scheme",
			input:   `[{"type":"sse","url":"ftp://mcp.example.com/sse"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateRemotes(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemotes() error = %v, wantErr %v", err, tt.wantErr)
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
		{name: "with tools flag", input: `{"tools":{"listChanged":true}}`},
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

func TestValidateTools(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// ── Allowed-empty cases ────────────────────────────────────────────
		// Unlike ValidateSkills, an empty tools field is legal: the publisher
		// may simply not declare tools up front. Each of these produces the
		// same "no tools declared" semantic and must NOT error.
		{name: "empty string → default to []", input: ``},
		{name: "literal null", input: `null`},
		{name: "empty array", input: `[]`},

		// ── Valid-shape cases ──────────────────────────────────────────────
		{
			name:  "single tool with name only",
			input: `[{"name":"read_file"}]`,
		},
		{
			name: "multiple tools with optional fields",
			input: `[
				{"name":"read_file","description":"Reads a file"},
				{"name":"write_file","description":"Writes a file","input_schema":{"type":"object","properties":{"path":{"type":"string"}}}},
				{"name":"list_dir","annotations":{"destructive":false}}
			]`,
		},
		{
			name:  "input_schema explicit null is ignored",
			input: `[{"name":"n","input_schema":null}]`,
		},

		// ── Shape violations ───────────────────────────────────────────────
		{
			name:    "not an array",
			input:   `{"name":"oops"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[{"name":`,
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `[{"description":"no name"}]`,
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   `[{"name":""}]`,
			wantErr: true,
		},
		{
			name:    "duplicate names",
			input:   `[{"name":"dup"},{"name":"dup"}]`,
			wantErr: true,
		},
		{
			name:    "input_schema is an array, not an object",
			input:   `[{"name":"n","input_schema":["type","object"]}]`,
			wantErr: true,
		},
		{
			name:    "input_schema is a string",
			input:   `[{"name":"n","input_schema":"not an object"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateTools(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTools(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProtocolVersions(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"single", []string{"2025-03-26"}, []string{"2025-03-26"}, false},
		{"sorted newest first", []string{"2024-11-05", "2025-06-18", "2025-03-26"}, []string{"2025-06-18", "2025-03-26", "2024-11-05"}, false},
		{"trimmed", []string{" 2025-03-26 "}, []string{"2025-03-26"}, false},
		{"nil", nil, nil, true},
		{"empty", []string{}, nil, true},
		{"blank entry", []string{"2025-03-26", ""}, nil, true},
		{"not a date", []string{"v1"}, nil, true},
		{"impossible date", []string{"2025-13-40"}, nil, true},
		{"duplicate", []string{"2025-03-26", "2025-03-26"}, nil, true},
		{"duplicate after trim", []string{"2025-03-26", "2025-03-26 "}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ValidateProtocolVersions(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateUsageMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		md      string
		wantErr bool
	}{
		{"empty", "", false},
		{"at limit counted in characters", strings.Repeat("é", domain.MaxUsageMarkdownLength), false},
		{"over limit", strings.Repeat("a", domain.MaxUsageMarkdownLength+1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateUsageMarkdown(tt.md); (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsageMarkdown() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
