// Package api embeds the OpenAPI 3.1 specification for serving at /openapi.yaml.
// The canonical source is /api/openapi.yaml at the repository root; this copy
// is kept in sync so the Go binary can serve the spec without filesystem access.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte
