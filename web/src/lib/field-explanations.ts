/**
 * Human-readable explanations for technical fields displayed in the UI.
 * Used by the TooltipInfo component on detail and listing pages.
 */

export const fieldExplanations: Record<string, string> = {
  // Runtimes / Transports
  stdio:
    "The server runs as a local process on your machine. Your MCP host starts it and communicates via stdin/stdout.",
  http:
    "The server is hosted remotely. Your MCP host connects via HTTP request/response.",
  sse:
    "Server-Sent Events. The server is hosted remotely. Your MCP host connects via HTTP and receives streaming responses.",
  streamable_http:
    "Streamable HTTP. The server is hosted remotely and uses HTTP with streaming support for bidirectional communication.",

  // Metadata fields
  protocol_versions:
    "The MCP protocol revisions this server supports. A host can connect when it supports at least one of them.",
  runtime:
    "How the MCP server runs: locally on your machine (stdio) or remotely via a network connection (SSE / Streamable HTTP).",
  mcp_authentication:
    "Remote MCP servers follow the MCP authorization spec — OAuth 2.1 with PKCE. Your client discovers the auth requirements at runtime from the server's protected-resource metadata.",

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
}

/**
 * Get the explanation for a field, or undefined if none exists.
 */
export function getFieldExplanation(field: string): string | undefined {
  return fieldExplanations[field] ?? fieldExplanations[field.toLowerCase()]
}
