package domain

import (
	"reflect"
	"testing"
)

func TestValidateTagColor(t *testing.T) {
	tests := []struct {
		name    string
		color   string
		wantErr bool
	}{
		{"gray is valid", "gray", false},
		{"pink is valid", "pink", false},
		{"teal is valid", "teal", false},
		{"empty is invalid", "", true},
		{"unknown color", "magenta", true},
		{"case-sensitive", "Gray", true},
		{"hex codes rejected", "#ff0000", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagColor(tt.color)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTagColor(%q) error = %v, wantErr %v", tt.color, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeVersionTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil stays nil", nil, nil},
		{"empty stays nil", []string{}, nil},
		{"only empty strings collapse to nil", []string{"", ""}, nil},
		{"duplicates collapse", []string{"free", "free"}, []string{"free"}},
		{"sorted output", []string{"zeta", "alpha"}, []string{"alpha", "zeta"}},
		{"empties dropped, rest kept", []string{"", "early-access", "", "free"}, []string{"early-access", "free"}},
		{
			"dedup and sort together",
			[]string{"free", "early-access", "free", "beta"},
			[]string{"beta", "early-access", "free"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeVersionTags(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeVersionTags(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
