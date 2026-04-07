package handlers

import (
	"net/http"
	"os"
)

// OAuthProtectedResource serves GET /.well-known/oauth-protected-resource.
// Per RFC 8707 and the MCP authorization spec, this advertises the resource
// identifier and its authorization servers so MCP clients can discover the IdP.
func OAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	resource := os.Getenv("PUBLIC_BASE_URL")
	if resource == "" {
		resource = "http://localhost:8081"
	}
	issuer := os.Getenv("OIDC_ISSUER")
	if issuer == "" {
		issuer = "http://keycloak:8080/realms/registry"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resource":              resource,
		"authorization_servers": []string{issuer},
		"bearer_methods_supported": []string{"header"},
		"resource_documentation": resource + "/docs",
	})
}
