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

const scalarBundleURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference"

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
    data-configuration='{"theme":"purple","withDefaultFonts":false}'
  ></script>
  <script src="` + scalarBundleURL + `"></script>
</body>
</html>`)

// docsContentSecurityPolicy overrides the API's deny-by-default policy with
// just what the Scalar bundle needs. It injects <style> elements at runtime,
// hence 'unsafe-inline' for styles only; it never needs inline script or eval.
const docsContentSecurityPolicy = "default-src 'none'; " +
	"script-src " + scalarBundleURL + "; " +
	"style-src 'unsafe-inline'; " +
	"img-src 'self' data: https:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// SwaggerUI handles GET /docs.
// It serves the Scalar API reference UI backed by /openapi.yaml.
func SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", docsContentSecurityPolicy)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docsHTML)
}
