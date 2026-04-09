package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
)

func TestValidateSkills(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "valid single skill",
			input: `[{"id":"search","name":"Search","description":"Searches the web","tags":["search"]}]`,
		},
		{
			name:  "valid skill with optional fields",
			input: `[{"id":"search","name":"Search","description":"Searches","tags":["search","web"],"examples":["search foo"],"inputModes":["text/plain"],"outputModes":["text/plain"]}]`,
		},
		{
			name:  "valid skill with empty tags array",
			input: `[{"id":"search","name":"Search","description":"Searches","tags":[]}]`,
		},
		{
			name:  "valid multiple skills",
			input: `[{"id":"a","name":"A","description":"Skill A","tags":[]},{"id":"b","name":"B","description":"Skill B","tags":["x"]}]`,
		},
		{
			name:    "empty raw input",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "not an array",
			input:   `{"id":"search"}`,
			wantErr: true,
		},
		{
			name:    "missing id",
			input:   `[{"name":"Search","description":"Searches","tags":[]}]`,
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `[{"id":"search","description":"Searches","tags":[]}]`,
			wantErr: true,
		},
		{
			name:    "missing description",
			input:   `[{"id":"search","name":"Search","tags":[]}]`,
			wantErr: true,
		},
		{
			name:    "missing tags field",
			input:   `[{"id":"search","name":"Search","description":"Searches"}]`,
			wantErr: true,
		},
		{
			name:    "empty string in tags",
			input:   `[{"id":"search","name":"Search","description":"Searches","tags":[""]}]`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[{bad}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateSkills(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkills() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAuthentication(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty is ok (public agent)", input: ``},
		{name: "null is ok", input: `null`},
		{name: "empty array is ok", input: `[]`},
		{name: "Bearer scheme", input: `[{"scheme":"Bearer"}]`},
		{name: "ApiKey scheme", input: `[{"scheme":"ApiKey"}]`},
		{name: "OAuth2 scheme", input: `[{"scheme":"OAuth2"}]`},
		{name: "OpenIdConnect scheme", input: `[{"scheme":"OpenIdConnect"}]`},
		{name: "multiple valid schemes", input: `[{"scheme":"Bearer"},{"scheme":"OAuth2"}]`},
		{
			name:    "unknown scheme",
			input:   `[{"scheme":"Basic"}]`,
			wantErr: true,
		},
		{
			name:    "empty scheme string",
			input:   `[{"scheme":""}]`,
			wantErr: true,
		},
		{
			name:    "missing scheme field",
			input:   `[{}]`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[{bad}]`,
			wantErr: true,
		},
		{
			name:    "not an array",
			input:   `{"scheme":"Bearer"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateAuthentication(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAuthentication() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
