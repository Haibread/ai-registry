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
import { ProtocolVersionsInput } from './protocol-versions-input'
import { ToolsEditor } from './tools-editor'
import { InstanceTagPicker } from './instance-tag-picker'
import { DirtyFormGuard } from '@/components/ui/dirty-form-guard'
import { collectInstanceTags } from '@/lib/use-instance-tags'
import type { components } from '@/lib/schema'

type MCPVersion = components['schemas']['MCPServerVersion']

// Kept in sync with the equivalents in pages/admin/mcp/new.tsx. The
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

// Shared styling for the monospace JSON textareas (tools / capabilities).
const jsonTextareaClass =
  'w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-xs ' +
  'placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring resize-y'

interface NewVersionFormProps {
  namespace: string
  slug: string
  /** Most recent existing version of the resource. Its values seed the form
   *  so authoring v(n+1) starts from v(n) — a small delta — instead of a
   *  blank slate. Omit on resources with no versions yet. */
  prefill?: MCPVersion
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

/** "1.2.3" → "1.2.4": suggested next version when seeding from a previous
 *  release. Empty (let the author type) when the prior version is unparsable. */
function bumpPatch(version: string | undefined): string {
  const m = version?.match(/^(\d+)\.(\d+)\.(\d+)/)
  if (!m) return ''
  return `${m[1]}.${m[2]}.${Number(m[3]) + 1}`
}

/** Pretty-print a JSON-object field for a textarea default; '' when absent/empty. */
function jsonDefault(obj: Record<string, unknown> | undefined): string {
  if (!obj || Object.keys(obj).length === 0) return ''
  return JSON.stringify(obj, null, 2)
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
export function NewVersionForm({ namespace, slug, prefill, onCreated, onCancel }: NewVersionFormProps) {
  const prefillPkg = prefill?.packages?.[0]
  const prefillRemote = prefill?.remotes?.[0]

  const [runtime, setRuntime] = useState<string>(prefill?.runtime ?? 'stdio')
  const [remoteUrl, setRemoteUrl] = useState(prefillRemote?.url ?? '')
  const [pkgRegistryType, setPkgRegistryType] = useState(prefillPkg?.registryType ?? 'npm')
  const [error, setError] = useState<string | null>(null)
  // Unsaved-changes guard — this form is the worst loss case (a
  // hand-built tools list dies on one stray sidebar click).
  const [dirty, setDirty] = useState(false)

  const mutation = useMutation({
    mutationFn: async (fd: FormData) => {
      setError(null)
      const version = (fd.get('version') as string).trim()
      if (!version) throw new Error('Version is required.')
      const instanceTags = collectInstanceTags(fd)

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

      // The remote-endpoint input only renders for non-stdio transports, so
      // fd.get returns null on stdio — hence the `?? ''` before trimming.
      const remoteUrl = ((fd.get('remote_url') as string | null) ?? '').trim()
      const remotes = remoteUrl ? [{ type: runtime, url: remoteUrl }] : []

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
          protocol_versions: JSON.parse(fd.get('protocol_versions') as string) as string[],
          ...(packages.length > 0 ? { packages } : {}),
          ...(remotes.length > 0 ? { remotes } : {}),
          ...(tools !== undefined ? { tools } : {}),
          ...(capabilities ? { capabilities } : {}),
          ...(instanceTags.length > 0 ? { tags: instanceTags } : {}),
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
      {prefill && (
        <p className="text-xs text-muted-foreground">
          Pre-filled from <span className="font-mono">v{prefill.version}</span> — adjust what
          changed and pick the new version number.
        </p>
      )}

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
            defaultValue={bumpPatch(prefill?.version)}
            required
            pattern="^\d+\.\d+\.\d+.*"
            title="Semantic version, e.g. 1.0.0"
          />
        </div>
      </div>

      <InstanceTagPicker defaultSelected={prefill?.tags ?? []} />

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

      {runtime !== 'stdio' && (
        <div className="space-y-1.5">
          <Label htmlFor="remote_url">Remote endpoint URL</Label>
          <Input
            id="remote_url"
            name="remote_url"
            type="url"
            placeholder="https://mcp.example.com/sse"
            value={remoteUrl}
            onChange={(e) => setRemoteUrl(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            Where clients reach the hosted server directly — no package
            needed for a purely remote server.
          </p>
        </div>
      )}

      <ProtocolVersionsInput
        defaultValue={prefill?.protocol_versions}
        discovery={{ namespace, transport: runtime, remoteUrl }}
      />

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
            <Input
              id="pkg_identifier"
              name="pkg_identifier"
              placeholder="@scope/name"
              defaultValue={prefillPkg?.identifier ?? ''}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pkg_version">Package version</Label>
            <Input
              id="pkg_version"
              name="pkg_version"
              placeholder="1.0.0 or latest"
              defaultValue={prefillPkg?.version ?? ''}
            />
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="pkg_url">Package URL</Label>
            <Input
              id="pkg_url"
              name="pkg_url"
              type="url"
              placeholder="https://…"
              defaultValue={prefillPkg?.transport?.url ?? ''}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pkg_registry_base_url">Registry base URL</Label>
            <Input
              id="pkg_registry_base_url"
              name="pkg_registry_base_url"
              type="url"
              placeholder="https://registry.npmjs.org"
              defaultValue={prefillPkg?.registryBaseUrl ?? ''}
            />
          </div>
        </div>
      </fieldset>

      <ToolsEditor
        name="tools"
        initialTools={prefill?.tools ?? []}
        discovery={{
          namespace,
          transport: runtime,
          remoteUrl,
        }}
      />

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
          defaultValue={jsonDefault(prefill?.capabilities)}
          className={jsonTextareaClass}
        />
      </div>

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
