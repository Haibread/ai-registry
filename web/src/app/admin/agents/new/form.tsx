"use client"

import { useActionState, useState } from "react"
import Link from "next/link"
import { ArrowLeft, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { createAgent, type CreateAgentState } from "../actions"
import type { components } from "@/lib/schema"

type Publisher = components["schemas"]["Publisher"]

const AUTH_SCHEME_OPTIONS = [
  { value: "Bearer", label: "Bearer (JWT / OAuth 2.0 access token)" },
  { value: "ApiKey", label: "ApiKey (static API key)" },
  { value: "OAuth2", label: "OAuth 2.0 (full flow)" },
  { value: "OpenIdConnect", label: "OpenID Connect" },
] as const

const MODE_OPTIONS = [
  { value: "text/plain", label: "text/plain" },
  { value: "application/json", label: "application/json" },
  { value: "image/png", label: "image/png" },
  { value: "text/csv", label: "text/csv" },
] as const

interface Props {
  publishers: Publisher[]
}

const initialState: CreateAgentState = {}

export function NewAgentForm({ publishers }: Props) {
  const [state, formAction, isPending] = useActionState(createAgent, initialState)

  const [namespace, setNamespace] = useState("")
  const [authScheme, setAuthScheme] = useState("")

  return (
    <div className="space-y-6 max-w-lg mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link href="/admin/agents" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Agents
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Agent</span>
      </nav>

      <h1 className="text-2xl font-bold">New Agent</h1>

      {state.error && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">
              {state.step === "version" ? "Agent created, but version creation failed" : "Failed to create agent"}
            </p>
            <p className="mt-0.5 text-destructive/80">{state.error}</p>
          </div>
        </div>
      )}

      <form action={formAction} className="space-y-6">
        {/* Hidden inputs for Radix Select values */}
        <input type="hidden" name="namespace" value={namespace} />
        <input type="hidden" name="auth_scheme" value={authScheme} />

        {/* ── Agent metadata ───────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Agent Details</CardTitle>
            <CardDescription>Basic metadata for the AI agent.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="namespace-select">
                Namespace (publisher) <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Select value={namespace} onValueChange={setNamespace} required>
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
                placeholder="my-agent"
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
              <Input id="name" name="name" placeholder="My Agent" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" placeholder="What this agent does…" />
            </div>
          </CardContent>
        </Card>

        {/* ── First version ────────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">First Version</CardTitle>
            <CardDescription>
              Leave "Version" blank to create the agent as a draft without a version.
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
                <Label htmlFor="protocol_version">A2A protocol version</Label>
                <Input
                  id="protocol_version"
                  name="protocol_version"
                  placeholder="0.2.1"
                  defaultValue="0.2.1"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="endpoint_url">
                Endpoint URL <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input
                id="endpoint_url"
                name="endpoint_url"
                type="url"
                placeholder="https://api.example.com/agent"
                aria-required="true"
              />
              <p className="text-xs text-muted-foreground">
                The A2A-compatible JSON-RPC endpoint for this agent version.
              </p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="auth-scheme-select">Authentication scheme</Label>
              <Select value={authScheme} onValueChange={setAuthScheme}>
                <SelectTrigger id="auth-scheme-select">
                  <SelectValue placeholder="None / public" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">None / public</SelectItem>
                  {AUTH_SCHEME_OPTIONS.map((a) => (
                    <SelectItem key={a.value} value={a.value}>
                      {a.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-2">
                <Label className="text-sm font-medium">Default input modes</Label>
                <div className="space-y-2">
                  {MODE_OPTIONS.map((m) => (
                    <div key={m.value} className="flex items-center gap-2">
                      <input
                        id={`input_mode_${m.value}`}
                        name={`input_mode_${m.value}`}
                        type="checkbox"
                        defaultChecked={m.value === "text/plain"}
                        className="h-4 w-4 rounded border border-input accent-primary"
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
                <Label className="text-sm font-medium">Default output modes</Label>
                <div className="space-y-2">
                  {MODE_OPTIONS.map((m) => (
                    <div key={m.value} className="flex items-center gap-2">
                      <input
                        id={`output_mode_${m.value}`}
                        name={`output_mode_${m.value}`}
                        type="checkbox"
                        defaultChecked={m.value === "text/plain"}
                        className="h-4 w-4 rounded border border-input accent-primary"
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
            </div>

            {/* ── First skill ──────────────────────────────────────────── */}
            <div className="rounded-md border border-dashed p-4 space-y-3">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                First Skill (optional)
              </p>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="skill_id">Skill ID</Label>
                  <Input id="skill_id" name="skill_id" placeholder="my-skill-id" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="skill_name">Skill name</Label>
                  <Input id="skill_name" name="skill_name" placeholder="My Skill" />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="skill_description">Skill description</Label>
                <Input id="skill_description" name="skill_description" placeholder="What this skill does…" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="skill_tags">Tags</Label>
                <Input id="skill_tags" name="skill_tags" placeholder="search, retrieval, summarization (comma-separated)" />
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
          {isPending ? "Creating…" : "Create Agent"}
        </Button>
      </form>
    </div>
  )
}
