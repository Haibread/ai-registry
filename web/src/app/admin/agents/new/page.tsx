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

export const metadata: Metadata = { title: "New Agent" }

const API_URL = process.env.API_URL ?? "http://localhost:8081"

/** Authentication scheme options — values must be in the backend allowlist. */
const AUTH_SCHEME_OPTIONS = [
  { value: "Bearer", label: "Bearer (JWT / OAuth 2.0 access token)" },
  { value: "ApiKey", label: "ApiKey (static API key)" },
  { value: "OAuth2", label: "OAuth 2.0 (full flow)" },
  { value: "OpenIdConnect", label: "OpenID Connect" },
] as const

/** MIME types used as input / output modes. */
const MODE_OPTIONS = [
  { value: "text/plain", label: "text/plain" },
  { value: "application/json", label: "application/json" },
  { value: "image/png", label: "image/png" },
  { value: "text/csv", label: "text/csv" },
] as const

const SELECT_CLS =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm " +
  "ring-offset-background focus-visible:outline-none focus-visible:ring-2 " +
  "focus-visible:ring-ring focus-visible:ring-offset-2"

export default async function NewAgentPage() {
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

    // ── Step 1: Create the agent ───────────────────────────────────────────
    const client = await getApiClient()
    const { data: agent, error: agentError } = await client.POST(
      "/api/v1/agents",
      {
        body: {
          namespace,
          slug,
          name,
          description: (formData.get("description") as string) || undefined,
        },
      }
    )
    if (agentError || !agent) return

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

    // ── Skills ────────────────────────────────────────────────────────────
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

    // ── Auth schemes ──────────────────────────────────────────────────────
    const authScheme = (formData.get("auth_scheme") as string).trim()
    const authentication = authScheme ? [{ scheme: authScheme }] : []

    // ── Input / output modes ─────────────────────────────────────────────
    const defaultInputModes = MODE_OPTIONS.map((m) => m.value).filter(
      (v) => formData.get(`input_mode_${v}`) === "on"
    )
    const defaultOutputModes = MODE_OPTIONS.map((m) => m.value).filter(
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
      redirect(`/admin/agents/${namespace}/${slug}`)
    }

    // ── Step 3: Publish the version if requested ───────────────────────────
    if (formData.get("publish") === "on") {
      await fetch(
        `${API_URL}/api/v1/agents/${namespace}/${slug}/versions/${version}/publish`,
        { method: "POST", headers }
      )
    }

    redirect(`/admin/agents/${namespace}/${slug}`)
  }

  return (
    <div className="space-y-6 max-w-lg">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/agents" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold">New Agent</h1>
      </div>

      <form action={create} className="space-y-4">
        {/* ── Agent metadata ───────────────────────────────────────────── */}
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
            placeholder="my-agent"
            pattern="^[a-z0-9-]+"
            required
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="name">Name *</Label>
          <Input id="name" name="name" placeholder="My Agent" required />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="description">Description</Label>
          <Input
            id="description"
            name="description"
            placeholder="What this agent does…"
          />
        </div>

        {/* ── First version ────────────────────────────────────────────── */}
        <Separator />
        <div>
          <h2 className="text-base font-semibold">First Version</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            Leave "Version" blank to create the agent in draft state without a
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
          <Label htmlFor="endpoint_url">Endpoint URL *</Label>
          <Input
            id="endpoint_url"
            name="endpoint_url"
            type="url"
            placeholder="https://api.example.com/agent"
          />
          <p className="text-xs text-muted-foreground">
            The A2A-compatible JSON-RPC endpoint for this agent version.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="protocol_version">A2A protocol version</Label>
          <Input
            id="protocol_version"
            name="protocol_version"
            placeholder="0.2.1"
            defaultValue="0.2.1"
          />
        </div>

        {/* ── Skill ────────────────────────────────────────────────────── */}
        <div className="rounded-md border border-dashed p-4 space-y-3">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
            First Skill (optional)
          </p>

          <div className="space-y-1.5">
            <Label htmlFor="skill_id">Skill ID</Label>
            <Input
              id="skill_id"
              name="skill_id"
              placeholder="my-skill-id (machine-readable)"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="skill_name">Skill name</Label>
            <Input
              id="skill_name"
              name="skill_name"
              placeholder="My Skill (human-readable)"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="skill_description">Skill description</Label>
            <Input
              id="skill_description"
              name="skill_description"
              placeholder="What this skill does…"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="skill_tags">Tags</Label>
            <Input
              id="skill_tags"
              name="skill_tags"
              placeholder="search, retrieval, summarization (comma-separated)"
            />
          </div>
        </div>

        {/* ── Authentication ───────────────────────────────────────────── */}
        <div className="space-y-1.5">
          <Label htmlFor="auth_scheme">Authentication scheme</Label>
          <select id="auth_scheme" name="auth_scheme" className={SELECT_CLS}>
            <option value="">None / public</option>
            {AUTH_SCHEME_OPTIONS.map((a) => (
              <option key={a.value} value={a.value}>
                {a.label}
              </option>
            ))}
          </select>
        </div>

        {/* ── Input / output modes ─────────────────────────────────────── */}
        <div className="space-y-2">
          <Label>Default input modes</Label>
          <div className="grid grid-cols-2 gap-2">
            {MODE_OPTIONS.map((m) => (
              <div key={m.value} className="flex items-center gap-2">
                <input
                  id={`input_mode_${m.value}`}
                  name={`input_mode_${m.value}`}
                  type="checkbox"
                  defaultChecked={m.value === "text/plain"}
                  className="h-4 w-4 rounded border border-input"
                />
                <Label
                  htmlFor={`input_mode_${m.value}`}
                  className="font-normal text-sm cursor-pointer"
                >
                  {m.label}
                </Label>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-2">
          <Label>Default output modes</Label>
          <div className="grid grid-cols-2 gap-2">
            {MODE_OPTIONS.map((m) => (
              <div key={m.value} className="flex items-center gap-2">
                <input
                  id={`output_mode_${m.value}`}
                  name={`output_mode_${m.value}`}
                  type="checkbox"
                  defaultChecked={m.value === "text/plain"}
                  className="h-4 w-4 rounded border border-input"
                />
                <Label
                  htmlFor={`output_mode_${m.value}`}
                  className="font-normal text-sm cursor-pointer"
                >
                  {m.label}
                </Label>
              </div>
            ))}
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
          Create Agent
        </Button>
      </form>
    </div>
  )
}
