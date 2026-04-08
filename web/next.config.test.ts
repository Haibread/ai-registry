/**
 * next.config.test.ts
 *
 * Verifies the Next.js rewrite rules that proxy well-known paths to the backend.
 *
 * These rules are the sole reason /.well-known/agent-card.json and
 * /agents/:ns/:slug/.well-known/agent-card.json return data instead of 404.
 * If they are removed or mis-typed, the agent card endpoints silently break
 * because Next.js would intercept the request and return 404 before the
 * backend is ever contacted.
 *
 * The config exports a plain object whose `rewrites` property is an async
 * function — we can call it directly without spinning up a Next.js server.
 */

// @vitest-environment node

import { describe, it, expect, beforeEach, afterEach } from "vitest"
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

    it("destination points to the API_URL backend (default localhost:8081)", async () => {
      const rewrites = await getRewrites()
      const rule = rewrites.find(
        (r) => r.source === "/agents/:namespace/:slug/.well-known/agent-card.json"
      )!
      // Default API_URL — the module was already imported so we check the shape
      expect(rule.destination).toMatch(/^https?:\/\//)
    })
  })

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

  describe("completeness", () => {
    it("has exactly 3 rewrite rules — one per well-known path", async () => {
      const rewrites = await getRewrites()
      expect(rewrites).toHaveLength(3)
    })

    it("all destinations are absolute URLs pointing to the same backend", async () => {
      const rewrites = await getRewrites()
      const origins = new Set(
        rewrites.map((r) => {
          try {
            // Strip the path — keep only scheme+host to verify they all target the same backend
            const url = new URL(r.destination.replace(/:[\w]+/g, "placeholder"))
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
