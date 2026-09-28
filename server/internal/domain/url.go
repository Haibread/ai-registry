package domain

import (
	"fmt"
	"net/url"
)

// MaxURLLength bounds every URL field an entry stores.
const MaxURLLength = 2048

// ValidateHTTPURL checks that raw is an absolute http or https URL with a
// host, no longer than MaxURLLength. Empty is accepted: callers enforce
// required-ness. field names the offending property in the error.
func ValidateHTTPURL(field, raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > MaxURLLength {
		return fmt.Errorf("%s must not exceed %d characters", field, MaxURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("%s %q is not a valid absolute http(s) URL", field, raw)
	}
	return nil
}
