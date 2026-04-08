import type { NextConfig } from "next"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      // ── REST API ────────────────────────────────────────────────────────────
      // Proxy all /api/v1/* requests to the Go backend.
      // This enables:
      //   • "JSON" links on server/agent cards (open raw API response in a new tab)
      //   • Direct browser access to the REST API from the same origin
      // Note: Next.js file-system routes (e.g. /api/auth/*) take precedence over
      // rewrites, so the auth routes are unaffected.
      {
        source: "/api/v1/:path*",
        destination: `${API_URL}/api/v1/:path*`,
      },
      // ── MCP wire-format endpoints ───────────────────────────────────────────
      // Proxy /v0/* so the MCP-registry-spec-compatible layer is reachable from
      // the same origin (useful for direct browser exploration and spec tooling).
      {
        source: "/v0/:path*",
        destination: `${API_URL}/v0/:path*`,
      },
      // ── Well-known / A2A ────────────────────────────────────────────────────
      // Proxy A2A agent card requests to the backend.
      // Next.js has no page at this path — without this rewrite the request
      // would match the [ns]/[slug] page and return 404.
      {
        source: "/agents/:namespace/:slug/.well-known/agent-card.json",
        destination: `${API_URL}/agents/:namespace/:slug/.well-known/agent-card.json`,
      },
      // Proxy the global registry agent card.
      {
        source: "/.well-known/agent-card.json",
        destination: `${API_URL}/.well-known/agent-card.json`,
      },
      // Proxy the MCP OAuth protected resource metadata endpoint.
      {
        source: "/.well-known/oauth-protected-resource",
        destination: `${API_URL}/.well-known/oauth-protected-resource`,
      },
    ]
  },
}

export default nextConfig
