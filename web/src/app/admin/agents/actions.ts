"use server"

import { redirect } from "next/navigation"
import { auth } from "@/auth"
import { getApiClient } from "@/lib/api-client"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

const MODE_VALUES = ["text/plain", "application/json", "image/png", "text/csv"] as const

export type CreateAgentState = {
  error?: string
  step?: string
}

export async function createAgent(
  _prev: CreateAgentState,
  formData: FormData
): Promise<CreateAgentState> {
  const namespace = formData.get("namespace") as string
  const slug = formData.get("slug") as string
  const name = formData.get("name") as string

  if (!namespace || !slug || !name) {
    return { error: "Namespace, slug, and name are required." }
  }

  // ── Step 1: Create the agent ───────────────────────────────────────────
  const client = await getApiClient()
  const { data: agent, error: agentError } = await client.POST("/api/v1/agents", {
    body: {
      namespace,
      slug,
      name,
      description: (formData.get("description") as string) || undefined,
    },
  })
  if (agentError || !agent) {
    const msg = (agentError as { title?: string } | undefined)?.title ?? "Failed to create agent."
    return { error: msg }
  }

  // ── Step 2: Create the first version (optional) ────────────────────────
  const version = (formData.get("version") as string).trim()
  if (!version) {
    redirect(`/admin/agents/${namespace}/${slug}`)
  }

  const endpointUrl = (formData.get("endpoint_url") as string).trim()
  if (!endpointUrl) {
    redirect(`/admin/agents/${namespace}/${slug}`)
  }

  const protocolVersion =
    (formData.get("protocol_version") as string).trim() || "0.2.1"

  const skillId = (formData.get("skill_id") as string).trim()
  const skillName = (formData.get("skill_name") as string).trim()
  const skillDescription = (formData.get("skill_description") as string).trim()
  const skillTagsRaw = (formData.get("skill_tags") as string).trim()
  const skillTags = skillTagsRaw
    ? skillTagsRaw.split(",").map((t) => t.trim()).filter(Boolean)
    : []

  const skills =
    skillId && skillName && skillDescription
      ? [{ id: skillId, name: skillName, description: skillDescription, tags: skillTags }]
      : []

  const authScheme = (formData.get("auth_scheme") as string).trim()
  const authentication = authScheme ? [{ scheme: authScheme }] : []

  const defaultInputModes = MODE_VALUES.filter(
    (v) => formData.get(`input_mode_${v}`) === "on"
  )
  const defaultOutputModes = MODE_VALUES.filter(
    (v) => formData.get(`output_mode_${v}`) === "on"
  )

  const session = await auth()
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (session?.accessToken) {
    headers["Authorization"] = `Bearer ${session.accessToken}`
  }

  const versionRes = await fetch(
    `${API_URL}/api/v1/agents/${namespace}/${slug}/versions`,
    {
      method: "POST",
      headers,
      body: JSON.stringify({
        version,
        endpoint_url: endpointUrl,
        protocol_version: protocolVersion,
        ...(skills.length > 0 ? { skills } : {}),
        ...(authentication.length > 0 ? { authentication } : {}),
        ...(defaultInputModes.length > 0 ? { default_input_modes: defaultInputModes } : {}),
        ...(defaultOutputModes.length > 0 ? { default_output_modes: defaultOutputModes } : {}),
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

  if (formData.get("publish") === "on") {
    const publishRes = await fetch(
      `${API_URL}/api/v1/agents/${namespace}/${slug}/versions/${version}/publish`,
      { method: "POST", headers }
    )
    if (!publishRes.ok) {
      redirect(`/admin/agents/${namespace}/${slug}?warn=publish-failed`)
    }
  }

  redirect(`/admin/agents/${namespace}/${slug}`)
}
