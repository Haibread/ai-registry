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

// docsHTML is the Scalar API reference UI page.
// Scalar is a CDN-hosted, dependency-free OpenAPI viewer that works with
// OpenAPI 3.1 specs. It points at the /openapi.yaml served by this server.
var docsHTML = []byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>AI Registry — API Reference</title>
  <style>body { margin: 0; }</style>
</head>
<body>
  <script
    id="api-reference"
    data-url="/openapi.yaml"
    data-configuration='{"theme":"purple"}'
  ></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`)

// SwaggerUI handles GET /docs.
// It serves the Scalar API reference UI backed by /openapi.yaml.
func SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docsHTML)
}
