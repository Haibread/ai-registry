import type { Metadata } from "next"
import { redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { getApiClient } from "@/lib/api-client"
import { auth } from "@/auth"

export const metadata: Metadata = { title: "New MCP Server" }

const API_URL = process.env.API_URL ?? "http://localhost:8081"

/** Transport options — values match the backend domain.Runtime enum. */
const TRANSPORT_OPTIONS = [
  { value: "stdio", label: "stdio (local process)" },
  { value: "sse", label: "SSE (HTTP Server-Sent Events)" },
  { value: "http", label: "HTTP (stateless HTTP)" },
  { value: "streamable_http", label: "Streamable HTTP (HTTP + streaming)" },
] as const

/** Registry / package manager options. */
const REGISTRY_OPTIONS = [
  { value: "npm", label: "npm" },
  { value: "pypi", label: "PyPI" },
  { value: "oci", label: "OCI (container)" },
  { value: "nuget", label: "NuGet" },
  { value: "mcpb", label: "mcpb" },
] as const

const SELECT_CLS =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm " +
  "ring-offset-background focus-visible:outline-none focus-visible:ring-2 " +
  "focus-visible:ring-ring focus-visible:ring-offset-2"

export default async function NewMCPServerPage() {
  const api = await getApiClient()
  const { data: pubData } = await api.GET("/api/v1/publishers", {
    params: { query: { limit: 100 } },
  })
  const publishers = pubData?.items ?? []

  async function create(formData: FormData) {
    "use server"

    const namespace = formData.get("namespace") as string
    const slug = formData.get("slug") as string
    const name = formData.get("name") as string

    // ── Step 1: Create the server ──────────────────────────────────────────
    const client = await getApiClient()
    const { data: server, error: serverError } = await client.POST(
      "/api/v1/mcp/servers",
      {
        body: {
          namespace,
          slug,
          name,
          description: (formData.get("description") as string) || undefined,
          homepage_url: (formData.get("homepage_url") as string) || undefined,
          repo_url: (formData.get("repo_url") as string) || undefined,
          license: (formData.get("license") as string) || undefined,
        },
      }
    )
    if (serverError || !server) return

    // ── Step 2: Create the first version (optional) ────────────────────────
    const version = (formData.get("version") as string).trim()
    if (!version) {
      redirect(`/admin/mcp/${namespace}/${slug}`)
    }

    const runtime = formData.get("runtime") as string
    const protocolVersion =
      (formData.get("protocol_version") as string).trim() || "2025-03-26"
    const pkgRegistryType = formData.get("pkg_registry_type") as string
    const pkgIdentifier = (formData.get("pkg_identifier") as string).trim()
    const pkgVersion = (formData.get("pkg_version") as string).trim()
    const pkgUrl = (formData.get("pkg_url") as string).trim()

    const packages =
      pkgIdentifier && pkgVersion
        ? [
            {
              registryType: pkgRegistryType,
              identifier: pkgIdentifier,
              version: pkgVersion,
              transport: {
                type: runtime,
                ...(pkgUrl ? { url: pkgUrl } : {}),
              },
            },
          ]
        : []

    const session = await auth()
    const headers: Record<string, string> = { "Content-Type": "application/json" }
    if (session?.accessToken) {
      headers["Authorization"] = `Bearer ${session.accessToken}`
    }

    const versionRes = await fetch(
      `${API_URL}/api/v1/mcp/servers/${namespace}/${slug}/versions`,
      {
        method: "POST",
        headers,
        body: JSON.stringify({
          version,
          runtime,
          protocol_version: protocolVersion,
          ...(packages.length > 0 ? { packages } : {}),
        }),
      }
    )
    if (!versionRes.ok) {
      redirect(`/admin/mcp/${namespace}/${slug}`)
    }

    // ── Step 3: Publish the version if requested ───────────────────────────
    if (formData.get("publish") === "on") {
      await fetch(
        `${API_URL}/api/v1/mcp/servers/${namespace}/${slug}/versions/${version}/publish`,
        { method: "POST", headers }
      )
    }

    redirect(`/admin/mcp/${namespace}/${slug}`)
  }

  return (
    <div className="space-y-6 max-w-lg">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/mcp" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold">New MCP Server</h1>
      </div>

      <form action={create} className="space-y-4">
        {/* ── Server metadata ──────────────────────────────────────────── */}
        <div className="space-y-1.5">
          <Label htmlFor="namespace">Namespace (publisher) *</Label>
          <select
            id="namespace"
            name="namespace"
            required
            className={SELECT_CLS}
          >
            <option value="">Select publisher…</option>
            {publishers.map((p) => (
              <option key={p.id} value={p.slug}>
                {p.slug} — {p.name}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="slug">Slug *</Label>
          <Input
            id="slug"
            name="slug"
            placeholder="my-server"
            pattern="^[a-z0-9-]+"
            required
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="name">Name *</Label>
          <Input id="name" name="name" placeholder="My MCP Server" required />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="description">Description</Label>
          <Input
            id="description"
            name="description"
            placeholder="What this server does…"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="repo_url">Repository URL</Label>
          <Input
            id="repo_url"
            name="repo_url"
            type="url"
            placeholder="https://github.com/…"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="homepage_url">Homepage URL</Label>
          <Input
            id="homepage_url"
            name="homepage_url"
            type="url"
            placeholder="https://…"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="license">License</Label>
          <Input id="license" name="license" placeholder="MIT" />
        </div>

        {/* ── First version ────────────────────────────────────────────── */}
        <Separator />
        <div>
          <h2 className="text-base font-semibold">First Version</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            Leave "Version" blank to create the server in draft state without a
            version.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="version">Version</Label>
          <Input
            id="version"
            name="version"
            placeholder="1.0.0"
            pattern="^\d+\.\d+\.\d+.*"
            title="Semantic version, e.g. 1.0.0"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="runtime">Transport type *</Label>
          <select id="runtime" name="runtime" required className={SELECT_CLS}>
            {TRANSPORT_OPTIONS.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
          <p className="text-xs text-muted-foreground">
            Use <strong>stdio</strong> for local process servers (npx/uvx).
            Use <strong>SSE</strong>, <strong>HTTP</strong>, or{" "}
            <strong>Streamable HTTP</strong> for remote servers — those require
            a package URL below.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="protocol_version">Protocol version</Label>
          <Input
            id="protocol_version"
            name="protocol_version"
            placeholder="2025-03-26"
            defaultValue="2025-03-26"
          />
        </div>

        {/* ── Package ──────────────────────────────────────────────────── */}
        <div className="rounded-md border border-dashed p-4 space-y-3">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
            Package (optional)
          </p>

          <div className="space-y-1.5">
            <Label htmlFor="pkg_registry_type">Registry</Label>
            <select
              id="pkg_registry_type"
              name="pkg_registry_type"
              className={SELECT_CLS}
            >
              {REGISTRY_OPTIONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="pkg_identifier">Package identifier</Label>
            <Input
              id="pkg_identifier"
              name="pkg_identifier"
              placeholder="@owner/package-name or owner/image:tag"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="pkg_version">Package version</Label>
            <Input
              id="pkg_version"
              name="pkg_version"
              placeholder="1.0.0 or latest"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="pkg_url">Package URL</Label>
            <Input
              id="pkg_url"
              name="pkg_url"
              type="url"
              placeholder="https://… (required for SSE / HTTP / Streamable HTTP)"
            />
            <p className="text-xs text-muted-foreground">
              Leave blank for stdio servers. Required for remote transports.
            </p>
          </div>
        </div>

        {/* ── Publish ──────────────────────────────────────────────────── */}
        <div className="flex items-center gap-2">
          <input
            id="publish"
            name="publish"
            type="checkbox"
            defaultChecked
            className="h-4 w-4 rounded border border-input"
          />
          <Label htmlFor="publish" className="cursor-pointer font-normal">
            Publish version immediately
          </Label>
        </div>

        <Button type="submit" className="w-full">
          Create MCP Server
        </Button>
      </form>
    </div>
  )
}
