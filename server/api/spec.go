// Package api embeds the OpenAPI 3.1 specification for serving at /openapi.yaml.
// The canonical source is server/api/openapi.yaml — edit that file directly.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte
