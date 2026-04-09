package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
)

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
