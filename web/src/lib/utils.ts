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
 * Derive the primary install command for a package entry.
 * Returns a ready-to-paste shell snippet.
 */
export function getInstallCommand(pkg: PackageEntry): string {
  const id = pkg.identifier
  switch (pkg.registryType.toLowerCase()) {
    case "npm":
      return `npx -y ${id}`
    case "pip":
    case "pypi":
      return `pip install ${id}`
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
