import type { NextConfig } from "next"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
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
