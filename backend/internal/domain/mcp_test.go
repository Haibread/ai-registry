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
			input: `[{"registryType":"docker","identifier":"myimage","version":"latest","transport":{"type":"http"}}]`,
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
