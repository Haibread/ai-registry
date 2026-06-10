import { useState } from 'react'
import { flushSync } from 'react-dom'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { authFetch } from '@/auth/tokens'
import { ToolsEditor } from './tools-editor'
import { DirtyFormGuard } from '@/components/ui/dirty-form-guard'

type Kind = 'mcp' | 'agent'

// Kept in sync with the equivalents in pages/admin/{mcp,agents}/new.tsx. The
// second copy is tolerable under the rule of three; extract to a shared module
// if a third version-authoring surface appears.
const TRANSPORT_OPTIONS = [
  { value: 'stdio', label: 'stdio (local process)' },
  { value: 'sse', label: 'SSE (HTTP Server-Sent Events)' },
  { value: 'http', label: 'HTTP (stateless HTTP)' },
  { value: 'streamable_http', label: 'Streamable HTTP (HTTP + streaming)' },
] as const

const REGISTRY_OPTIONS = [
  { value: 'npm', label: 'npm' },
  { value: 'pypi', label: 'PyPI' },
  { value: 'oci', label: 'OCI (container)' },
  { value: 'nuget', label: 'NuGet' },
  { value: 'mcpb', label: 'mcpb' },
] as const

const AUTH_SCHEME_OPTIONS = [
  { value: 'Bearer', label: 'Bearer (JWT / OAuth 2.0 access token)' },
  { value: 'ApiKey', label: 'ApiKey (static API key)' },
  { value: 'OAuth2', label: 'OAuth 2.0 (full flow)' },
  { value: 'OpenIdConnect', label: 'OpenID Connect' },
] as const

const MODE_VALUES = ['text/plain', 'application/json', 'image/png', 'text/csv'] as const

// Shared styling for the monospace JSON textareas (tools / capabilities).
const jsonTextareaClass =
  'w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-xs ' +
  'placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring resize-y'

interface NewVersionFormProps {
  kind: Kind
  namespace: string
  slug: string
  /** Called after a draft version is created so the parent can refetch + close. */
  onCreated: (version: string) => void
  onCancel: () => void
}

/** Pull a friendly message out of a non-OK problem+json response. */
async function problemTitle(res: Response, fallback: string): Promise<string> {
  try {
    const body = await res.json()
    if (body?.detail) return body.detail as string
    if (body?.title) return body.title as string
  } catch {
    /* body not JSON — keep fallback */
  }
  return `${fallback} (HTTP ${res.status})`
}

/** Parse an optional JSON-object field; throws a friendly error on malformed input. */
function parseJsonObject(raw: string, label: string): Record<string, unknown> | undefined {
  const t = raw.trim()
  if (t === '') return undefined
  let parsed: unknown
  try {
    parsed = JSON.parse(t)
  } catch {
    throw new Error(`${label} must be valid JSON.`)
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object.`)
  }
  return parsed as Record<string, unknown>
}

// NewVersionForm authors a new DRAFT version of an existing resource and POSTs
// it to the versions endpoint. It deliberately stops at "draft": submitting for
// review (and approve/reject) stay on the existing Submit button / review queue,
// so the create → submit → approve lifecycle has one owner per step.
export function NewVersionForm({ kind, namespace, slug, onCreated, onCancel }: NewVersionFormProps) {
  const [runtime, setRuntime] = useState('stdio')
  const [pkgRegistryType, setPkgRegistryType] = useState('npm')
  const [authScheme, setAuthScheme] = useState('_none')
  const [error, setError] = useState<string | null>(null)
  // Unsaved-changes guard (P2.5) — this form is the worst loss case (a
  // hand-built tools list dies on one stray sidebar click).
  const [dirty, setDirty] = useState(false)

  const mutation = useMutation({
    mutationFn: async (fd: FormData) => {
      setError(null)
      const version = (fd.get('version') as string).trim()
      if (!version) throw new Error('Version is required.')
      const protocolVersion = (fd.get('protocol_version') as string).trim()

      if (kind === 'mcp') {
        const pkgIdentifier = (fd.get('pkg_identifier') as string).trim()
        const pkgVersion = (fd.get('pkg_version') as string).trim()
        const pkgUrl = (fd.get('pkg_url') as string).trim()
        const pkgRegistryBaseUrl = (fd.get('pkg_registry_base_url') as string).trim()
        const packages =
          pkgIdentifier && pkgVersion
            ? [
                {
                  registryType: pkgRegistryType,
                  identifier: pkgIdentifier,
                  version: pkgVersion,
                  ...(pkgRegistryBaseUrl ? { registryBaseUrl: pkgRegistryBaseUrl } : {}),
                  transport: { type: runtime, ...(pkgUrl ? { url: pkgUrl } : {}) },
                },
              ]
            : []

        // Parse tools client-side so structural mistakes surface here rather
        // than as a generic 422; the backend validator re-checks on write.
        const toolsRaw = ((fd.get('tools') as string) ?? '').trim()
        let tools: unknown = undefined
        if (toolsRaw !== '') {
          try {
            tools = JSON.parse(toolsRaw)
          } catch {
            throw new Error('Tools field must be valid JSON (an array of tool objects).')
          }
          if (!Array.isArray(tools)) throw new Error('Tools field must be a JSON array.')
        }

        const capabilities = parseJsonObject((fd.get('capabilities') as string) ?? '', 'Capabilities')

        const res = await authFetch(`/api/v1/mcp/servers/${namespace}/${slug}/versions`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            version,
            runtime,
            protocol_version: protocolVersion || '2025-03-26',
            ...(packages.length > 0 ? { packages } : {}),
            ...(tools !== undefined ? { tools } : {}),
            ...(capabilities ? { capabilities } : {}),
          }),
        })
        if (!res.ok) throw new Error(await problemTitle(res, 'Failed to create version.'))
        return version
      }

      // ── agent ──────────────────────────────────────────────────────────
      const endpointUrl = (fd.get('endpoint_url') as string).trim()
      if (!endpointUrl) throw new Error('Endpoint URL is required.')

      const skillId = (fd.get('skill_id') as string).trim()
      const skillName = (fd.get('skill_name') as string).trim()
      const skillDescription = (fd.get('skill_description') as string).trim()
      const skillTagsRaw = (fd.get('skill_tags') as string).trim()
      const skillTags = skillTagsRaw
        ? skillTagsRaw.split(',').map((t) => t.trim()).filter(Boolean)
        : []
      const skillExamplesRaw = (fd.get('skill_examples') as string).trim()
      const skillExamples = skillExamplesRaw
        ? skillExamplesRaw.split('\n').map((s) => s.trim()).filter(Boolean)
        : []
      const skills =
        skillId && skillName && skillDescription
          ? [
              {
                id: skillId,
                name: skillName,
                description: skillDescription,
                tags: skillTags,
                ...(skillExamples.length > 0 ? { examples: skillExamples } : {}),
              },
            ]
          : []

      const authentication = authScheme && authScheme !== '_none' ? [{ scheme: authScheme }] : []
      const defaultInputModes = MODE_VALUES.filter((v) => fd.get(`input_mode_${v}`) === 'on')
      const defaultOutputModes = MODE_VALUES.filter((v) => fd.get(`output_mode_${v}`) === 'on')

      const documentationUrl = (fd.get('documentation_url') as string).trim()
      const iconUrl = (fd.get('icon_url') as string).trim()
      const providerOrg = (fd.get('provider_organization') as string).trim()
      const providerUrl = (fd.get('provider_url') as string).trim()
      const provider =
        providerOrg || providerUrl
          ? {
              ...(providerOrg ? { organization: providerOrg } : {}),
              ...(providerUrl ? { url: providerUrl } : {}),
            }
          : undefined
      const capabilities = parseJsonObject((fd.get('capabilities') as string) ?? '', 'Capabilities')

      const res = await authFetch(`/api/v1/agents/${namespace}/${slug}/versions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          version,
          endpoint_url: endpointUrl,
          protocol_version: protocolVersion || '0.2.1',
          ...(skills.length > 0 ? { skills } : {}),
          ...(authentication.length > 0 ? { authentication } : {}),
          ...(defaultInputModes.length > 0 ? { default_input_modes: defaultInputModes } : {}),
          ...(defaultOutputModes.length > 0 ? { default_output_modes: defaultOutputModes } : {}),
          ...(provider ? { provider } : {}),
          ...(documentationUrl ? { documentation_url: documentationUrl } : {}),
          ...(iconUrl ? { icon_url: iconUrl } : {}),
          ...(capabilities ? { capabilities } : {}),
        }),
      })
      if (!res.ok) throw new Error(await problemTitle(res, 'Failed to create version.'))
      return version
    },
    onSuccess: (version) => {
      // flushSync so a parent that navigates from onCreated isn't blocked
      // by the guard this form just satisfied.
      flushSync(() => setDirty(false))
      toast.success(`v${version} created as draft`)
      onCreated(version)
    },
    onError: (e: Error) => setError(e.message),
  })

  return (
    <form
      className="space-y-4 border rounded-lg p-4"
      onChange={() => setDirty(true)}
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate(new FormData(e.currentTarget))
      }}
    >
      <DirtyFormGuard when={dirty} />
      <h3 className="text-base font-semibold">New version</h3>

      {error && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="version">
            Version <span className="text-destructive" aria-hidden="true">*</span>
          </Label>
          <Input
            id="version"
            name="version"
            placeholder="1.0.0"
            required
            pattern="^\d+\.\d+\.\d+.*"
            title="Semantic version, e.g. 1.0.0"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="protocol_version">Protocol version</Label>
          <Input
            id="protocol_version"
            name="protocol_version"
            placeholder={kind === 'mcp' ? '2025-03-26' : '0.2.1'}
          />
        </div>
      </div>

      {kind === 'mcp' ? (
        <>
          <div className="space-y-1.5">
            <Label htmlFor="runtime-select">Transport</Label>
            <Select value={runtime} onValueChange={(v) => { setRuntime(v); setDirty(true) }}>
              <SelectTrigger id="runtime-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TRANSPORT_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <fieldset className="space-y-3 rounded-md border p-3">
            <legend className="px-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Package (optional)
            </legend>
            <div className="space-y-1.5">
              <Label htmlFor="pkg_registry_type-select">Registry</Label>
              <Select value={pkgRegistryType} onValueChange={(v) => { setPkgRegistryType(v); setDirty(true) }}>
                <SelectTrigger id="pkg_registry_type-select">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {REGISTRY_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="pkg_identifier">Package identifier</Label>
                <Input id="pkg_identifier" name="pkg_identifier" placeholder="@scope/name" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pkg_version">Package version</Label>
                <Input id="pkg_version" name="pkg_version" placeholder="1.0.0 or latest" />
              </div>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="pkg_url">Package URL</Label>
                <Input id="pkg_url" name="pkg_url" type="url" placeholder="https://…" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pkg_registry_base_url">Registry base URL</Label>
                <Input
                  id="pkg_registry_base_url"
                  name="pkg_registry_base_url"
                  type="url"
                  placeholder="https://registry.npmjs.org"
                />
              </div>
            </div>
          </fieldset>

          <ToolsEditor name="tools" />

          <div className="space-y-1.5">
            <Label htmlFor="capabilities" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Capabilities (optional, JSON)
            </Label>
            <textarea
              id="capabilities"
              name="capabilities"
              rows={4}
              spellCheck={false}
              placeholder={'{\n  "tools": { "listChanged": true }\n}'}
              className={jsonTextareaClass}
            />
          </div>
        </>
      ) : (
        <>
          <div className="space-y-1.5">
            <Label htmlFor="endpoint_url">
              Endpoint URL <span className="text-destructive" aria-hidden="true">*</span>
            </Label>
            <Input
              id="endpoint_url"
              name="endpoint_url"
              type="url"
              placeholder="https://agent.example.com"
              required
            />
          </div>

          <fieldset className="space-y-3 rounded-md border p-3">
            <legend className="px-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Skill (optional)
            </legend>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="skill_id">Skill ID</Label>
                <Input id="skill_id" name="skill_id" placeholder="summarize" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="skill_name">Skill name</Label>
                <Input id="skill_name" name="skill_name" placeholder="Summarize text" />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="skill_description">Skill description</Label>
              <Input id="skill_description" name="skill_description" placeholder="What the skill does" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="skill_tags">Skill tags (comma-separated)</Label>
              <Input id="skill_tags" name="skill_tags" placeholder="text, nlp" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="skill_examples">Skill examples (one per line)</Label>
              <textarea
                id="skill_examples"
                name="skill_examples"
                rows={3}
                spellCheck={false}
                placeholder={'Summarize this article\nGive me a TL;DR of the docs'}
                className={jsonTextareaClass}
              />
            </div>
          </fieldset>

          <div className="space-y-1.5">
            <Label htmlFor="auth-scheme-select">Authentication</Label>
            <Select value={authScheme} onValueChange={(v) => { setAuthScheme(v); setDirty(true) }}>
              <SelectTrigger id="auth-scheme-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">None</SelectItem>
                {AUTH_SCHEME_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <fieldset className="space-y-2">
              <legend className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Input modes
              </legend>
              {MODE_VALUES.map((v) => (
                <label key={v} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    name={`input_mode_${v}`}
                    defaultChecked={v === 'text/plain'}
                  />
                  <span className="font-mono">{v}</span>
                </label>
              ))}
            </fieldset>
            <fieldset className="space-y-2">
              <legend className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Output modes
              </legend>
              {MODE_VALUES.map((v) => (
                <label key={v} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    name={`output_mode_${v}`}
                    defaultChecked={v === 'text/plain'}
                  />
                  <span className="font-mono">{v}</span>
                </label>
              ))}
            </fieldset>
          </div>

          <fieldset className="space-y-3 rounded-md border p-3">
            <legend className="px-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Provider &amp; metadata (optional)
            </legend>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="provider_organization">Provider organization</Label>
                <Input id="provider_organization" name="provider_organization" placeholder="Acme Inc." />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="provider_url">Provider URL</Label>
                <Input id="provider_url" name="provider_url" type="url" placeholder="https://acme.example.com" />
              </div>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="documentation_url">Documentation URL</Label>
                <Input id="documentation_url" name="documentation_url" type="url" placeholder="https://docs.example.com" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="icon_url">Icon URL</Label>
                <Input id="icon_url" name="icon_url" type="url" placeholder="https://…/icon.png" />
              </div>
            </div>
          </fieldset>

          <div className="space-y-1.5">
            <Label htmlFor="capabilities" className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Capabilities (optional, JSON)
            </Label>
            <textarea
              id="capabilities"
              name="capabilities"
              rows={4}
              spellCheck={false}
              placeholder={'{\n  "streaming": true,\n  "pushNotifications": false\n}'}
              className={jsonTextareaClass}
            />
          </div>
        </>
      )}

      <p className="text-xs text-muted-foreground">
        Creates a draft. Use <span className="font-medium">Submit</span> on the version row to send it
        for review.
      </p>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={mutation.isPending}>
          {mutation.isPending ? 'Creating…' : 'Create version'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
