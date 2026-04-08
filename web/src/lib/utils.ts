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
