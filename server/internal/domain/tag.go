package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// InstanceTag is one entry of the instance-wide tag vocabulary curated by
// Server Admins. Publishers tick tags from this vocabulary when creating a
// version; the ticked slugs are frozen into the immutable version row. A tag
// referenced by any version can therefore only be deactivated (hidden from
// new publishes), never deleted — hard delete is reserved for unused tags.
//
// The JSON tags are the wire format of the /api/v1/tags endpoints — they
// must stay in sync with the InstanceTag schema in server/api/openapi.yaml.
type InstanceTag struct {
	ID          string    `json:"-"` // internal ULID; the slug is the public identity
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color,omitempty"`
	Active      bool      `json:"active"`
	Managed     bool      `json:"managed"` // defined in server configuration → read-only via the API
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// tagColors lists the badge color hints the UI knows how to render, in the
// order shown to admins. Kept in sync with the `color` enum on the
// InstanceTag schema in server/api/openapi.yaml.
var tagColors = []string{
	"gray", "red", "orange", "yellow", "green",
	"teal", "blue", "indigo", "purple", "pink",
}

// ValidateTagColor checks that color is one of the supported badge colors.
func ValidateTagColor(color string) error {
	for _, c := range tagColors {
		if color == c {
			return nil
		}
	}
	return fmt.Errorf("color %q is not valid (must be one of: %s)", color, strings.Join(tagColors, ", "))
}

// NormalizeVersionTags prepares the tag slugs ticked on a version for
// storage: empty strings are dropped, duplicates collapse, and the result is
// sorted so equal selections always serialize identically. Returns nil when
// nothing remains.
func NormalizeVersionTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}
