"use client"

import { useActionState, useState } from "react"
import Link from "next/link"
import { ArrowLeft, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { createMCPServer, type CreateMCPServerState } from "../actions"
import type { components } from "@/lib/schema"

type Publisher = components["schemas"]["Publisher"]

const TRANSPORT_OPTIONS = [
  { value: "stdio", label: "stdio (local process)" },
  { value: "sse", label: "SSE (HTTP Server-Sent Events)" },
  { value: "http", label: "HTTP (stateless HTTP)" },
  { value: "streamable_http", label: "Streamable HTTP (HTTP + streaming)" },
] as const

const REGISTRY_OPTIONS = [
  { value: "npm", label: "npm" },
  { value: "pypi", label: "PyPI" },
  { value: "oci", label: "OCI (container)" },
  { value: "nuget", label: "NuGet" },
  { value: "mcpb", label: "mcpb" },
] as const

interface Props {
  publishers: Publisher[]
}

const initialState: CreateMCPServerState = {}

export function NewMCPServerForm({ publishers }: Props) {
  const [state, formAction, isPending] = useActionState(createMCPServer, initialState)

  // Track select values so hidden inputs stay in sync with Radix Select
  const [namespace, setNamespace] = useState("")
  const [runtime, setRuntime] = useState("stdio")
  const [pkgRegistryType, setPkgRegistryType] = useState("npm")

  return (
    <div className="space-y-6 max-w-lg">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link href="/admin/mcp" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          MCP Servers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Server</span>
      </nav>

      <h1 className="text-2xl font-bold">New MCP Server</h1>

      {state.error && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">
              {state.step === "version" ? "Server created, but version creation failed" : "Failed to create server"}
            </p>
            <p className="mt-0.5 text-destructive/80">{state.error}</p>
          </div>
        </div>
      )}

      <form action={formAction} className="space-y-6">
        {/* Hidden inputs for Radix Select values */}
        <input type="hidden" name="namespace" value={namespace} />
        <input type="hidden" name="runtime" value={runtime} />
        <input type="hidden" name="pkg_registry_type" value={pkgRegistryType} />

        {/* ── Server metadata ──────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Server Details</CardTitle>
            <CardDescription>Basic metadata for the MCP server.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="namespace-select">
                Namespace (publisher) <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Select
                required
                value={namespace}
                onValueChange={setNamespace}
              >
                <SelectTrigger id="namespace-select" aria-required="true">
                  <SelectValue placeholder="Select publisher…" />
                </SelectTrigger>
                <SelectContent>
                  {publishers.map((p) => (
                    <SelectItem key={p.id} value={p.slug}>
                      {p.slug} — {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="slug">
                Slug <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input
                id="slug"
                name="slug"
                placeholder="my-server"
                pattern="^[a-z0-9-]+"
                title="Lowercase letters, numbers, and hyphens only"
                required
                aria-required="true"
              />
              <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only.</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="name">
                Name <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input id="name" name="name" placeholder="My MCP Server" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" placeholder="What this server does…" />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="repo_url">Repository URL</Label>
                <Input id="repo_url" name="repo_url" type="url" placeholder="https://github.com/…" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="homepage_url">Homepage URL</Label>
                <Input id="homepage_url" name="homepage_url" type="url" placeholder="https://…" />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="license">License</Label>
              <Input id="license" name="license" placeholder="MIT" />
            </div>
          </CardContent>
        </Card>

        {/* ── First version ────────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">First Version</CardTitle>
            <CardDescription>
              Leave "Version" blank to create the server as a draft without a version.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
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
                <Label htmlFor="protocol_version">Protocol version</Label>
                <Input
                  id="protocol_version"
                  name="protocol_version"
                  placeholder="2025-03-26"
                  defaultValue="2025-03-26"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="runtime-select">
                Transport type <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Select value={runtime} onValueChange={setRuntime} required>
                <SelectTrigger id="runtime-select" aria-required="true">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TRANSPORT_OPTIONS.map((t) => (
                    <SelectItem key={t.value} value={t.value}>
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Use <strong>stdio</strong> for local process servers (npx/uvx).
                Use <strong>SSE</strong>, <strong>HTTP</strong>, or <strong>Streamable HTTP</strong> for remote servers — those require a package URL below.
              </p>
            </div>

            {/* ── Package ──────────────────────────────────────────────── */}
            <div className="rounded-md border border-dashed p-4 space-y-3">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                Package (optional)
              </p>

              <div className="space-y-1.5">
                <Label htmlFor="pkg_registry_type-select">Registry</Label>
                <Select value={pkgRegistryType} onValueChange={setPkgRegistryType}>
                  <SelectTrigger id="pkg_registry_type-select">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {REGISTRY_OPTIONS.map((r) => (
                      <SelectItem key={r.value} value={r.value}>
                        {r.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="pkg_identifier">Package identifier</Label>
                  <Input
                    id="pkg_identifier"
                    name="pkg_identifier"
                    placeholder="@owner/package-name"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="pkg_version">Package version</Label>
                  <Input id="pkg_version" name="pkg_version" placeholder="1.0.0 or latest" />
                </div>
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

            <div className="flex items-center gap-2">
              <input
                id="publish"
                name="publish"
                type="checkbox"
                defaultChecked
                className="h-4 w-4 rounded border border-input accent-primary"
              />
              <Label htmlFor="publish" className="cursor-pointer font-normal">
                Publish version immediately
              </Label>
            </div>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full" disabled={isPending || !namespace}>
          {isPending ? "Creating…" : "Create MCP Server"}
        </Button>
      </form>
    </div>
  )
}
