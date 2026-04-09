package store

import (
	"testing"
)

func TestPrefixTSQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"single partial word", "t", "t:*"},
		{"single complete word", "test", "test:*"},
		{"two words", "test server", "test:* & server:*"},
		{"pipe becomes separator", "foo|bar", "foo:* & bar:*"},
		{"amp becomes separator", "foo&bar", "foo:* & bar:*"},
		{"strips bang prefix", "!foo", "foo:*"},
		{"strips parens", "(foo)", "foo:*"},
		{"colon becomes separator", "foo:bar", "foo:* & bar:*"},
		{"quote becomes separator", "foo'bar", "foo:* & bar:*"},
		{"mixed operators and words", "test! (server|client)", "test:* & server:* & client:*"},
		{"extra whitespace", "  foo   bar  ", "foo:* & bar:*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prefixTSQuery(tt.input)
			if got != tt.want {
				t.Errorf("prefixTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
