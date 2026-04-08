"use server"

import { redirect } from "next/navigation"
import { auth } from "@/auth"
import { getApiClient } from "@/lib/api-client"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

export type CreateMCPServerState = {
  error?: string
  step?: string
}

export async function createMCPServer(
  _prev: CreateMCPServerState,
  formData: FormData
): Promise<CreateMCPServerState> {
  const namespace = formData.get("namespace") as string
  const slug = formData.get("slug") as string
  const name = formData.get("name") as string

  if (!namespace || !slug || !name) {
    return { error: "Namespace, slug, and name are required." }
  }

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
  if (serverError || !server) {
    const msg = (serverError as { title?: string } | undefined)?.title ?? "Failed to create server."
    return { error: msg }
  }

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
    let msg = `Failed to create version (HTTP ${versionRes.status}).`
    try {
      const body = await versionRes.json()
      if (body?.title) msg = body.title
    } catch {}
    return { error: msg, step: "version" }
  }

  // ── Step 3: Publish the version if requested ───────────────────────────
  if (formData.get("publish") === "on") {
    const publishRes = await fetch(
      `${API_URL}/api/v1/mcp/servers/${namespace}/${slug}/versions/${version}/publish`,
      { method: "POST", headers }
    )
    if (!publishRes.ok) {
      // Server created and version created — just warn, don't block redirect
      redirect(`/admin/mcp/${namespace}/${slug}?warn=publish-failed`)
    }
  }

  redirect(`/admin/mcp/${namespace}/${slug}`)
}
