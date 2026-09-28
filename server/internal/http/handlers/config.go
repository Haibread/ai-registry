package handlers

import "net/http"

// ConfigJSON returns a handler for GET /config.json — the public runtime config
// the browser SPA reads on first load. The SPA is not an OIDC client, so this
// ships no OIDC coordinates, only the feature flags that tell the SPA which
// sign-in buttons to render.
//
// The endpoint is intentionally public (no auth): the flags are not secrets and
// the SPA must read them before a user has authenticated.
func ConfigJSON(oidcEnabled, localLoginEnabled bool) http.HandlerFunc {
	type response struct {
		OIDCEnabled       bool `json:"oidc_enabled"`
		LocalLoginEnabled bool `json:"local_login_enabled"`
	}
	payload := response{
		OIDCEnabled:       oidcEnabled,
		LocalLoginEnabled: localLoginEnabled,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, payload)
	}
}
