package handlers

import (
	"net/http"

	registryapi "github.com/haibread/ai-registry/api"
)

// OpenAPISpec handles GET /openapi.yaml.
// It serves the embedded OpenAPI 3.1 specification.
func OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(registryapi.Spec)
}
