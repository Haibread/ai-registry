import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"
import type { components } from "@/lib/schema"

type PackageEntry = components["schemas"]["PackageEntry"]

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Format an ISO date string into a human-readable date. */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  })
}

/**
 * Format a non-negative count for compact display on cards.
 * Examples: 0 → "0", 42 → "42", 1234 → "1.2k", 15000 → "15k", 1_500_000 → "1.5M".
 */
export function formatCount(n: number | null | undefined): string {
  if (n == null || n < 0) return "0"
  if (n < 1000) return String(n)
  if (n < 1_000_000) {
    const k = n / 1000
    return k < 10 ? `${k.toFixed(1).replace(/\.0$/, "")}k` : `${Math.round(k)}k`
  }
  const m = n / 1_000_000
  return m < 10 ? `${m.toFixed(1).replace(/\.0$/, "")}M` : `${Math.round(m)}M`
}

/** Returns true for transport types that connect to a remote URL rather than running a local process. */
export function isRemoteTransport(type: string): boolean {
  return type === "sse" || type === "http" || type === "streamable_http"
}

/**
 * Derive the primary install / connect command for a package entry.
 *
 * For URL-based transports (SSE, HTTP, Streamable HTTP) the relevant artifact
 * is the endpoint URL, not a package install command.  For stdio transports
 * the artifact is the shell command to run the server locally.
 */
export function getInstallCommand(pkg: PackageEntry): string {
  // Remote transports — the thing you paste into your MCP client is the URL
  if (isRemoteTransport(pkg.transport.type) && pkg.transport.url) {
    return pkg.transport.url
  }

  const id = pkg.identifier
  switch (pkg.registryType.toLowerCase()) {
    case "npm":
      return `npx -y ${id}`
    case "pip":
    case "pypi":
      return `pip install ${id}`
    case "oci":
    case "docker":
      return `docker run ${id}`
    case "gem":
      return `gem install ${id}`
    case "go":
      return `go install ${id}`
    default:
      return id
  }
}

/**
 * Count the tools declared by an MCP server's `capabilities` blob.
 *
 * `capabilities` is typed as free-form JSON in the spec (decision F), so
 * the field is `{[key: string]: unknown}` on the generated type. We cannot
 * assume `tools` is an array — publishers may omit it, ship it as an
 * object, or encode it some other way.
 *
 * Returns:
 *   - `null` when the count is *unknown* (field absent, wrong shape).
 *     Cards MUST hide the chip in this case — showing "0 tools" for a
 *     server that just didn't populate the field would falsely advertise
 *     a capability-free server.
 *   - a non-negative integer when `capabilities.tools` is a valid array.
 *     `0` is still a "known" value and distinct from `null`; individual
 *     call sites decide whether to render a chip for the zero case.
 */
export function countMcpTools(capabilities: unknown): number | null {
  if (capabilities == null || typeof capabilities !== "object") return null
  const caps = capabilities as Record<string, unknown>
  const tools = caps.tools
  if (!Array.isArray(tools)) return null
  return tools.length
}

/**
 * Map a registryType to a short ecosystem label used in badges.
 */
export function ecosystemLabel(registryType: string): string {
  switch (registryType.toLowerCase()) {
    case "npm": return "npm"
    case "pip":
    case "pypi": return "pip"
    case "docker": return "docker"
    case "gem": return "gem"
    case "go": return "go"
    default: return registryType
  }
}
