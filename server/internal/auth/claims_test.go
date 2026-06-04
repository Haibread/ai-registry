package auth_test

import (
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
)

func TestOIDCClaims_HasGroup(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		query  string
		want   bool
	}{
		{name: "match", groups: []string{"a", "b", "c"}, query: "b", want: true},
		{name: "no match", groups: []string{"a", "b"}, query: "c", want: false},
		{name: "empty groups", groups: nil, query: "a", want: false},
		{name: "empty query never matches", groups: []string{"a", ""}, query: "", want: false},
		{name: "case-sensitive", groups: []string{"Claude-Team"}, query: "claude-team", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &auth.OIDCClaims{Groups: tt.groups}
			if got := c.HasGroup(tt.query); got != tt.want {
				t.Errorf("HasGroup(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
