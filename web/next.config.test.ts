/**
 * next.config.test.ts
 *
 * Verifies the Next.js rewrite rules that proxy backend paths through the
 * Next.js server.  Without these rules:
 *
 *   • The "JSON" buttons on MCP-server and agent cards return 404 because
 *     /api/v1/… has no file-system handler in Next.js.
 *   • /.well-known/agent-card.json and the per-agent variant return 404
 *     because Next.js would match its own [ns]/[slug] page instead.
 *   • /v0/… MCP wire-format endpoints are unreachable from the same origin.
 *
 * The config exports a plain object whose `rewrites` property is an async
 * function — we call it directly without spinning up a Next.js server.
 */

// @vitest-environment node

import { describe, it, expect, afterEach } from "vitest"
import nextConfig from "./next.config"

type Rewrite = { source: string; destination: string }

async function getRewrites(): Promise<Rewrite[]> {
  if (typeof nextConfig.rewrites !== "function") {
    throw new Error("nextConfig.rewrites is not a function")
  }
  const result = await nextConfig.rewrites()
  // rewrites() can return an array OR { beforeFiles, afterFiles, fallback }
  if (Array.isArray(result)) return result as Rewrite[]
  return [
    ...((result.beforeFiles ?? []) as Rewrite[]),
    ...((result.afterFiles ?? []) as Rewrite[]),
    ...((result.fallback ?? []) as Rewrite[]),
  ]
}

describe("next.config rewrites", () => {
  const originalEnv = process.env.API_URL

  afterEach(() => {
    if (originalEnv === undefined) {
      delete process.env.API_URL
    } else {
      process.env.API_URL = originalEnv
    }
  })

  // ── REST API proxy (/api/v1/*) ─────────────────────────────────────────────

  describe("/api/v1 proxy rewrite", () => {
    it("has a rewrite rule for /api/v1/:path*", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/api/v1/:path*")
      expect(rule, "missing rewrite for /api/v1 — JSON buttons on cards will 404").toBeDefined()
    })

    it("destination proxies /api/v1/:path* to the backend", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/api/v1/:path*")!
      expect(rule.destination).toContain("/api/v1/")
      expect(rule.destination).toContain(":path*")
    })

    it("destination is an absolute URL (has scheme)", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/api/v1/:path*")!
      expect(rule.destination).toMatch(/^https?:\/\//)
    })
  })

  // ── MCP wire-format proxy (/v0/*) ──────────────────────────────────────────

  describe("/v0 proxy rewrite", () => {
    it("has a rewrite rule for /v0/:path*", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/v0/:path*")
      expect(rule, "missing rewrite for /v0 — MCP wire-format endpoints unreachable from same origin").toBeDefined()
    })

    it("destination proxies /v0/:path* to the backend", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/v0/:path*")!
      expect(rule.destination).toContain("/v0/")
      expect(rule.destination).toContain(":path*")
    })

    it("destination is an absolute URL (has scheme)", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/v0/:path*")!
      expect(rule.destination).toMatch(/^https?:\/\//)
    })
  })

  // ── Per-agent A2A card ─────────────────────────────────────────────────────

  describe("per-agent card rewrite", () => {
    it("has a rewrite rule for /agents/:namespace/:slug/.well-known/agent-card.json", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find(
        (r) => r.source === "/agents/:namespace/:slug/.well-known/agent-card.json"
      )
      expect(rule, "missing rewrite for per-agent card path").toBeDefined()
    })

    it("destination preserves the :namespace and :slug params", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find(
        (r) => r.source === "/agents/:namespace/:slug/.well-known/agent-card.json"
      )!
      expect(rule.destination).toContain(":namespace")
      expect(rule.destination).toContain(":slug")
      expect(rule.destination).toContain("/.well-known/agent-card.json")
    })

    it("destination is an absolute URL pointing to the backend", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find(
        (r) => r.source === "/agents/:namespace/:slug/.well-known/agent-card.json"
      )!
      expect(rule.destination).toMatch(/^https?:\/\//)
    })
  })

  // ── Global registry agent card ─────────────────────────────────────────────

  describe("global registry agent card rewrite", () => {
    it("has a rewrite rule for /.well-known/agent-card.json", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/.well-known/agent-card.json")
      expect(rule, "missing rewrite for global agent card").toBeDefined()
    })

    it("destination ends with /.well-known/agent-card.json", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/.well-known/agent-card.json")!
      expect(rule.destination).toContain("/.well-known/agent-card.json")
    })
  })

  // ── MCP OAuth protected-resource metadata ──────────────────────────────────

  describe("MCP OAuth protected-resource rewrite", () => {
    it("has a rewrite rule for /.well-known/oauth-protected-resource", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/.well-known/oauth-protected-resource")
      expect(rule, "missing rewrite for MCP OAuth protected-resource metadata").toBeDefined()
    })

    it("destination ends with /.well-known/oauth-protected-resource", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find((r) => r.source === "/.well-known/oauth-protected-resource")!
      expect(rule.destination).toContain("/.well-known/oauth-protected-resource")
    })
  })

  // ── Completeness ───────────────────────────────────────────────────────────

  describe("completeness", () => {
    it("has exactly 5 rewrite rules", async () => {
      const rewrites = await getRewrites()
      // /api/v1/:path*, /v0/:path*, per-agent card, global card, oauth-protected-resource
      expect(rewrites).toHaveLength(5)
    })

    it("all destinations are absolute URLs pointing to the same backend", async () => {
      const rewrites = await getRewrites()
      const origins = new Set(
        rewrites.map((r) => {
          try {
            const url = new URL(r.destination.replace(/:[\w]+|\*/g, "placeholder"))
            return url.origin
          } catch {
            return r.destination
          }
        })
      )
      expect(origins.size).toBe(1)
    })

    it("no rule has an empty source or destination", async () => {
      const rewrites = await getRewrites()
      for (const rule of rewrites) {
        expect(rule.source, `empty source in rule ${JSON.stringify(rule)}`).not.toBe("")
        expect(rule.destination, `empty destination in rule ${JSON.stringify(rule)}`).not.toBe("")
      }
    })
  })
})
