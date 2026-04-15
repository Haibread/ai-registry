/**
 * utils.test.ts
 *
 * Unit tests for src/lib/utils.ts — pure utility functions with no
 * network or DOM dependencies.
 */

// @vitest-environment node

import { describe, it, expect } from "vitest"
import { cn, formatDate, formatCount, getInstallCommand, ecosystemLabel, isRemoteTransport, countMcpTools } from "./utils"
import type { components } from "@/lib/schema"

type PackageEntry = components["schemas"]["PackageEntry"]
type Transport = components["schemas"]["PackageTransport"]

function makePkg(registryType: string, identifier: string, version = "1.0.0", transport: Transport = { type: "stdio" }): PackageEntry {
  return { registryType, identifier, version, transport }
}

// ── cn ────────────────────────────────────────────────────────────────────────

describe("cn", () => {
  it("returns a single class unchanged", () => {
    expect(cn("text-red-500")).toBe("text-red-500")
  })

  it("merges multiple classes", () => {
    expect(cn("p-4", "m-2")).toBe("p-4 m-2")
  })

  it("deduplicates Tailwind conflicts (last wins)", () => {
    // tailwind-merge resolves p-4 vs p-2: last one wins
    expect(cn("p-4", "p-2")).toBe("p-2")
  })

  it("ignores falsy values", () => {
    expect(cn("text-sm", false, undefined, null, "font-bold")).toBe(
      "text-sm font-bold"
    )
  })

  it("supports conditional objects", () => {
    expect(cn({ "text-red-500": true, "text-blue-500": false })).toBe(
      "text-red-500"
    )
  })

  it("supports array inputs", () => {
    expect(cn(["flex", "items-center"])).toBe("flex items-center")
  })

  it("returns empty string for no inputs", () => {
    expect(cn()).toBe("")
  })

  it("returns empty string for all-falsy inputs", () => {
    expect(cn(false, undefined, null)).toBe("")
  })
})

// ── formatDate ────────────────────────────────────────────────────────────────

describe("formatDate", () => {
  it("formats an ISO date string to human-readable form", () => {
    // 2025-01-15 should render as "Jan 15, 2025"
    const result = formatDate("2025-01-15T00:00:00Z")
    expect(result).toMatch(/Jan/)
    expect(result).toMatch(/15/)
    expect(result).toMatch(/2025/)
  })

  it("handles date-only strings", () => {
    const result = formatDate("2024-06-01")
    expect(result).toMatch(/2024/)
    expect(result).toMatch(/Jun/)
  })

  it("includes month, day, and year", () => {
    // Use noon UTC to avoid date shifting across timezone boundaries.
    const result = formatDate("2023-12-15T12:00:00Z")
    // Dec 15, 2023
    expect(result).toMatch(/Dec/)
    expect(result).toMatch(/15/)
    expect(result).toMatch(/2023/)
  })

  it("produces a non-empty string for any valid date", () => {
    const dates = [
      "2020-02-29T12:00:00Z", // leap day (noon to avoid TZ shift)
      "2000-01-15T12:00:00Z",
      "1999-06-15T12:00:00Z",
      "2099-07-04T12:00:00.000Z",
    ]
    for (const d of dates) {
      expect(formatDate(d).length).toBeGreaterThan(0)
    }
  })
})

// ── getInstallCommand ─────────────────────────────────────────────────────────

describe("getInstallCommand", () => {
  it("npm → npx -y <id>", () => {
    expect(getInstallCommand(makePkg("npm", "@modelcontextprotocol/server-filesystem"))).toBe(
      "npx -y @modelcontextprotocol/server-filesystem"
    )
  })

  it("pip → pip install <id>", () => {
    expect(getInstallCommand(makePkg("pip", "mcp-server-git"))).toBe("pip install mcp-server-git")
  })

  it("pypi → pip install <id>", () => {
    expect(getInstallCommand(makePkg("pypi", "mcp-server-git"))).toBe("pip install mcp-server-git")
  })

  it("docker → docker run <id>", () => {
    expect(getInstallCommand(makePkg("docker", "ghcr.io/acme/my-mcp"))).toBe(
      "docker run ghcr.io/acme/my-mcp"
    )
  })

  it("gem → gem install <id>", () => {
    expect(getInstallCommand(makePkg("gem", "my-mcp-server"))).toBe("gem install my-mcp-server")
  })

  it("go → go install <id>", () => {
    expect(getInstallCommand(makePkg("go", "github.com/acme/mcp-server@latest"))).toBe(
      "go install github.com/acme/mcp-server@latest"
    )
  })

  it("unknown registryType → falls back to identifier", () => {
    expect(getInstallCommand(makePkg("custom", "my-identifier"))).toBe("my-identifier")
  })

  it("oci → docker run <id>", () => {
    expect(getInstallCommand(makePkg("oci", "ghcr.io/acme/mcp-server"))).toBe(
      "docker run ghcr.io/acme/mcp-server"
    )
  })

  it("is case-insensitive for registryType", () => {
    expect(getInstallCommand(makePkg("NPM", "some-pkg"))).toBe("npx -y some-pkg")
    expect(getInstallCommand(makePkg("PyPI", "some-pkg"))).toBe("pip install some-pkg")
    expect(getInstallCommand(makePkg("Docker", "some/image"))).toBe("docker run some/image")
  })

  it("SSE transport with URL → returns the endpoint URL", () => {
    const pkg = makePkg("npm", "@acme/mcp", "1.0.0", { type: "sse", url: "https://api.acme.com/mcp/sse" })
    expect(getInstallCommand(pkg)).toBe("https://api.acme.com/mcp/sse")
  })

  it("HTTP transport with URL → returns the endpoint URL", () => {
    const pkg = makePkg("npm", "@acme/mcp", "1.0.0", { type: "http", url: "https://api.acme.com/mcp" })
    expect(getInstallCommand(pkg)).toBe("https://api.acme.com/mcp")
  })

  it("streamable_http transport with URL → returns the endpoint URL", () => {
    const pkg = makePkg("npm", "@acme/mcp", "1.0.0", { type: "streamable_http", url: "https://api.acme.com/mcp/stream" })
    expect(getInstallCommand(pkg)).toBe("https://api.acme.com/mcp/stream")
  })

  it("remote transport without URL → falls back to registry install command", () => {
    const pkg = makePkg("npm", "@acme/mcp", "1.0.0", { type: "sse" })
    expect(getInstallCommand(pkg)).toBe("npx -y @acme/mcp")
  })
})

// ── isRemoteTransport ─────────────────────────────────────────────────────────

describe("isRemoteTransport", () => {
  it("stdio → false", () => { expect(isRemoteTransport("stdio")).toBe(false) })
  it("sse → true",   () => { expect(isRemoteTransport("sse")).toBe(true) })
  it("http → true",  () => { expect(isRemoteTransport("http")).toBe(true) })
  it("streamable_http → true", () => { expect(isRemoteTransport("streamable_http")).toBe(true) })
  it("unknown type → false",   () => { expect(isRemoteTransport("unknown")).toBe(false) })
})

// ── ecosystemLabel ────────────────────────────────────────────────────────────

describe("ecosystemLabel", () => {
  it("npm → npm", () => { expect(ecosystemLabel("npm")).toBe("npm") })
  it("pip → pip", () => { expect(ecosystemLabel("pip")).toBe("pip") })
  it("pypi → pip", () => { expect(ecosystemLabel("pypi")).toBe("pip") })
  it("docker → docker", () => { expect(ecosystemLabel("docker")).toBe("docker") })
  it("gem → gem", () => { expect(ecosystemLabel("gem")).toBe("gem") })
  it("go → go", () => { expect(ecosystemLabel("go")).toBe("go") })
  it("unknown → returns as-is", () => { expect(ecosystemLabel("cargo")).toBe("cargo") })
  it("is case-insensitive", () => {
    expect(ecosystemLabel("NPM")).toBe("npm")
    expect(ecosystemLabel("DOCKER")).toBe("docker")
  })
})

// ── formatCount ───────────────────────────────────────────────────────────────

describe("formatCount", () => {
  it("returns '0' for null and undefined", () => {
    expect(formatCount(null)).toBe("0")
    expect(formatCount(undefined)).toBe("0")
  })

  it("returns '0' for negative numbers", () => {
    expect(formatCount(-5)).toBe("0")
  })

  it("returns the raw string for values under 1000", () => {
    expect(formatCount(0)).toBe("0")
    expect(formatCount(1)).toBe("1")
    expect(formatCount(42)).toBe("42")
    expect(formatCount(999)).toBe("999")
  })

  it("uses one decimal in the k range when under 10k", () => {
    expect(formatCount(1000)).toBe("1k")
    expect(formatCount(1234)).toBe("1.2k")
    expect(formatCount(9999)).toBe("10k") // rounds up
  })

  it("rounds to whole k between 10k and 1M", () => {
    expect(formatCount(15000)).toBe("15k")
    expect(formatCount(999499)).toBe("999k")
  })

  it("switches to M for values >= 1M", () => {
    expect(formatCount(1_000_000)).toBe("1M")
    expect(formatCount(1_500_000)).toBe("1.5M")
    expect(formatCount(15_000_000)).toBe("15M")
  })
})

// ── countMcpTools ─────────────────────────────────────────────────────────────

describe("countMcpTools", () => {
  it("returns null for null / undefined / non-object capabilities", () => {
    expect(countMcpTools(null)).toBeNull()
    expect(countMcpTools(undefined)).toBeNull()
    expect(countMcpTools("not an object")).toBeNull()
    expect(countMcpTools(42)).toBeNull()
  })

  it("returns null when the tools field is absent", () => {
    expect(countMcpTools({})).toBeNull()
    expect(countMcpTools({ resources: [] })).toBeNull()
  })

  it("returns null when the tools field is the wrong shape", () => {
    // Publishers may ship tools as an object (indexed by name) or as a
    // string — neither is something we can count reliably, so the chip
    // must hide rather than show "0 tools".
    expect(countMcpTools({ tools: {} })).toBeNull()
    expect(countMcpTools({ tools: "search,code" })).toBeNull()
    expect(countMcpTools({ tools: 5 })).toBeNull()
  })

  it("returns 0 for an empty tools array (known-empty ≠ unknown)", () => {
    expect(countMcpTools({ tools: [] })).toBe(0)
  })

  it("returns the array length for a populated tools array", () => {
    expect(countMcpTools({ tools: [{ name: "a" }] })).toBe(1)
    expect(
      countMcpTools({ tools: [{ name: "a" }, { name: "b" }, { name: "c" }] })
    ).toBe(3)
  })

  it("does not care about other top-level capability fields", () => {
    const caps = {
      tools: [{ name: "a" }, { name: "b" }],
      resources: [{ uri: "a" }],
      prompts: [{ name: "p" }],
      logging: {},
    }
    expect(countMcpTools(caps)).toBe(2)
  })
})
