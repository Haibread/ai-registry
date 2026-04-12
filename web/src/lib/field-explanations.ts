/**
 * Human-readable explanations for technical fields displayed in the UI.
 * Used by the TooltipInfo component on detail and listing pages.
 */

export const fieldExplanations: Record<string, string> = {
  // Runtimes / Transports
  stdio:
    "The server runs as a local process on your machine. Your MCP host starts it and communicates via stdin/stdout.",
  sse:
    "Server-Sent Events. The server is hosted remotely. Your MCP host connects via HTTP and receives streaming responses.",
  streamable_http:
    "Streamable HTTP. The server is hosted remotely and uses HTTP with streaming support for bidirectional communication.",

  // Metadata fields
  protocol_version:
    "The version of the MCP protocol this server implements. Hosts and servers must agree on a compatible protocol version.",
  a2a_protocol_version:
    "The version of the A2A (Agent-to-Agent) protocol this agent implements.",
  runtime:
    "How the MCP server runs: locally on your machine (stdio) or remotely via a network connection (SSE / Streamable HTTP).",
  endpoint_url:
    "The URL where this agent or server is reachable. Your client sends requests to this address.",

  // Package ecosystems
  npm: "A Node.js package available via the npm registry. Install with npx or npm.",
  pip: "A Python package available via PyPI. Install with pip.",
  pypi: "A Python package available via PyPI. Install with pip.",
  docker: "A container image available via Docker Hub or a compatible registry.",
  go: "A Go module. Install with go install.",
  gem: "A Ruby gem. Install with gem install.",

  // Visibility
  public: "Visible to everyone browsing the registry.",
  private: "Only visible to authenticated admins.",

  // Input/Output modes
  "text/plain": "Plain text input or output.",
  "image/*": "Image data (PNG, JPEG, etc.).",
  "application/json": "Structured JSON data.",
  "audio/*": "Audio data.",
  "video/*": "Video data.",

  // Auth schemes
  Bearer: "Authenticate with a Bearer token in the Authorization header.",
  ApiKey: "Authenticate with an API key, typically in a header or query parameter.",
  OAuth2: "Authenticate using the OAuth 2.0 authorization flow.",
  OpenIdConnect: "Authenticate using OpenID Connect (OIDC), an identity layer on top of OAuth 2.0.",
}

/**
 * Get the explanation for a field, or undefined if none exists.
 */
export function getFieldExplanation(field: string): string | undefined {
  return fieldExplanations[field] ?? fieldExplanations[field.toLowerCase()]
}
