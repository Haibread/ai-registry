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
func ConfigJSON(oidcIssuer, oidcClientID string) http.HandlerFunc {
	type response struct {
		OIDCIssuer   string `json:"oidc_issuer"`
		OIDCClientID string `json:"oidc_client_id"`
	}
	payload := response{
		OIDCIssuer:   oidcIssuer,
		OIDCClientID: oidcClientID,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, payload)
	}
}
