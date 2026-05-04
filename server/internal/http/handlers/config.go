package handlers

import "net/http"

// ConfigJSON returns a handler for GET /config.json.
// It serves the public client-side configuration required by the browser SPA
// to bootstrap its OIDC client at runtime — eliminating the need to bake
// OIDC coordinates into the Docker image as build-time environment variables.
//
// The endpoint is intentionally public (no auth required): the OIDC issuer
// and client ID are not secrets, and the SPA must be able to call it before
// a user has authenticated.
//
// authStorage controls where the SPA persists OIDC tokens. "session" (the
// default) scopes tokens to the browser tab, which limits XSS-exfiltration
// blast radius. "local" is an E2E escape hatch: Playwright's storageState
// captures localStorage across contexts — do not use in production.
func ConfigJSON(oidcIssuer, oidcClientID, authStorage string) http.HandlerFunc {
	if authStorage != "local" {
		authStorage = "session"
	}
	type response struct {
		OIDCIssuer   string `json:"oidc_issuer"`
		OIDCClientID string `json:"oidc_client_id"`
		AuthStorage  string `json:"auth_storage"`
	}
	payload := response{
		OIDCIssuer:   oidcIssuer,
		OIDCClientID: oidcClientID,
		AuthStorage:  authStorage,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, payload)
	}
}
