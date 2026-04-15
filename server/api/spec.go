// Package api embeds the OpenAPI 3.1 specification for serving at /openapi.yaml,
// plus the pinned A2A Agent Card JSON Schema used by conformance tests.
// The canonical sources are server/api/openapi.yaml and
// server/api/a2a-agent-card.schema.json — edit those files directly.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte

// A2AAgentCardSchema is the JSON Schema for the A2A Agent Card document shape,
// pinned to the a2a-project/a2a June 2025 commit per CLAUDE.md Resolved
// Decision G. Consumed by internal/http/handlers tests to assert that every
// card the registry emits (per-agent and global) conforms to the spec.
//
//go:embed a2a-agent-card.schema.json
var A2AAgentCardSchema []byte
