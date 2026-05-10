package handlers

import (
	"log/slog"
	"net/http"

	"github.com/haibread/ai-registry/internal/problem"
)

// OAuthProtectedResource serves GET /.well-known/oauth-protected-resource.
// Per RFC 8707 and the MCP authorization spec, this advertises the resource
// identifier and its authorization servers so MCP clients can discover the IdP.
//
// Both `publicBaseURL` and `oidcIssuer` come from the config layer
// (env + YAML + default). When `publicBaseURL` is empty the handler returns
// HTTP 500 — silently advertising `localhost` to external consumers is worse
// than a hard failure that surfaces the misconfiguration. This mirrors the
// fail-loud pattern used by GlobalAgentCard.
func OAuthProtectedResource(publicBaseURL, oidcIssuer string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if publicBaseURL == "" {
			if logger != nil {
				logger.ErrorContext(r.Context(),
					"PUBLIC_BASE_URL is not set; cannot advertise oauth-protected-resource",
				)
			}
			problem.Write(w, http.StatusInternalServerError, "misconfiguration",
				"PUBLIC_BASE_URL is not set", r.URL.Path)
			return
		}
		writeJSON(w, r, http.StatusOK, map[string]any{
			"resource":                 publicBaseURL,
			"authorization_servers":    []string{oidcIssuer},
			"bearer_methods_supported": []string{"header"},
			"resource_documentation":   publicBaseURL + "/docs",
		})
	}
}
