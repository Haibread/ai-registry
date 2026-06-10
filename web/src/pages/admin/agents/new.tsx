import { useState } from 'react'
import { flushSync } from 'react-dom'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useAuthClient } from '@/lib/api-client'
import { usePublisher } from '@/auth/PublisherContext'
import { usePermissions } from '@/auth/useMe'
import { authFetch } from '@/auth/tokens'
import { problemMessage } from '@/lib/utils'
import { SlugField } from '@/components/admin/slug-field'
import { DirtyFormGuard } from '@/components/ui/dirty-form-guard'

const AUTH_SCHEME_OPTIONS = [
  { value: 'Bearer', label: 'Bearer (JWT / OAuth 2.0 access token)' },
  { value: 'ApiKey', label: 'ApiKey (static API key)' },
  { value: 'OAuth2', label: 'OAuth 2.0 (full flow)' },
  { value: 'OpenIdConnect', label: 'OpenID Connect' },
] as const

const MODE_OPTIONS = [
  { value: 'text/plain', label: 'text/plain' },
  { value: 'application/json', label: 'application/json' },
  { value: 'image/png', label: 'image/png' },
  { value: 'text/csv', label: 'text/csv' },
] as const

const MODE_VALUES = ['text/plain', 'application/json', 'image/png', 'text/csv'] as const

type CreateError = { step?: string; message: string }

export default function AdminAgentNew() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const api = useAuthClient()

  // Only offer publishers the caller can author on — derived from their grants
  // (GET /api/v1/me, via PublisherContext) so the dropdown cannot drift from the
  // server's RBAC. A Server Admin sees every publisher; a member only theirs.
  const { publishers: scopedPublishers, currentSlug } = usePublisher()
  const perms = usePermissions()
  const publishers = scopedPublishers.filter((p) => perms.canEdit(p.slug))

  // Pre-select the publisher the admin area is currently scoped to, when the
  // caller can author on it — so creating from a publisher's context lands on
  // that publisher. Derived (not synced via an effect) so it tracks the
  // editable list as it loads asynchronously for a Server Admin; an explicit
  // pick always wins. Empty when scoped to All-publishers / no edit rights,
  // which keeps the submit gate until one is chosen.
  const [picked, setPicked] = useState<string | null>(null)
  const defaultNamespace =
    currentSlug && publishers.some((p) => p.slug === currentSlug) ? currentSlug : ''
  const namespace = picked ?? defaultNamespace
  // Radix only learns an item's label once its content has been opened, so a
  // programmatically pre-selected value would otherwise render as the
  // placeholder. Drive the trigger label ourselves from the chosen publisher.
  const selectedPublisher = publishers.find((p) => p.slug === namespace)
  const [authScheme, setAuthScheme] = useState('_none')
  const [formError, setFormError] = useState<CreateError | null>(null)
  // Unsaved-changes guard (P2.5): any input change marks the form dirty;
  // a successful create clears it (synchronously, so the redirect isn't
  // blocked by the guard it just satisfied).
  const [dirty, setDirty] = useState(false)

  // Publishing is a reviewer action; an editor's version goes through the
  // review queue instead. The checkbox below adapts its label and behavior so
  // the form never promises an outcome the caller's role cannot deliver (J1).
  const canReview = perms.canReview(namespace)

  const mutation = useMutation({
    mutationFn: async (formData: FormData) => {
      const ns = namespace
      const slug = (formData.get('slug') as string).trim()
      const name = (formData.get('name') as string).trim()

      if (!ns || !slug || !name) {
        throw { step: undefined, message: 'Namespace, slug, and name are required.' }
      }

      // Step 1: Create agent
      const { data: agent, error: agentError } = await api.POST('/api/v1/agents', {
        body: {
          namespace: ns,
          slug,
          name,
          description: (formData.get('description') as string) || undefined,
        },
      })
      if (agentError || !agent) {
        throw { step: undefined, message: problemMessage(agentError, 'Failed to create agent.') }
      }

      // Step 2: Create version (optional). Version + endpoint are paired: you
      // either provide both (to author an initial version) or neither. Filling
      // only one is almost always a mistake, so surface it instead of silently
      // skipping version creation.
      const version = (formData.get('version') as string).trim()
      const endpointUrl = (formData.get('endpoint_url') as string).trim()
      if ((version && !endpointUrl) || (!version && endpointUrl)) {
        throw {
          step: 'version',
          message: 'Provide both a version and an endpoint URL to create a version, or leave both blank.',
        }
      }
      if (!version || !endpointUrl) {
        return { namespace: ns, slug }
      }

      const protocolVersion = (formData.get('protocol_version') as string).trim() || '0.2.1'
      const skillId = (formData.get('skill_id') as string).trim()
      const skillName = (formData.get('skill_name') as string).trim()
      const skillDescription = (formData.get('skill_description') as string).trim()
      const skillTagsRaw = (formData.get('skill_tags') as string).trim()
      const skillTags = skillTagsRaw ? skillTagsRaw.split(',').map((t) => t.trim()).filter(Boolean) : []

      const skills = skillId && skillName && skillDescription
        ? [{ id: skillId, name: skillName, description: skillDescription, tags: skillTags }]
        : []

      const authentication = (authScheme && authScheme !== '_none') ? [{ scheme: authScheme }] : []

      const defaultInputModes = MODE_VALUES.filter((v) => formData.get(`input_mode_${v}`) === 'on')
      const defaultOutputModes = MODE_VALUES.filter((v) => formData.get(`output_mode_${v}`) === 'on')

      const versionRes = await authFetch(`/api/v1/agents/${ns}/${slug}/versions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          version,
          endpoint_url: endpointUrl,
          protocol_version: protocolVersion,
          ...(skills.length > 0 ? { skills } : {}),
          ...(authentication.length > 0 ? { authentication } : {}),
          ...(defaultInputModes.length > 0 ? { default_input_modes: defaultInputModes } : {}),
          ...(defaultOutputModes.length > 0 ? { default_output_modes: defaultOutputModes } : {}),
        }),
      })
      if (!versionRes.ok) {
        const fallback = `Failed to create version (HTTP ${versionRes.status}).`
        let msg = fallback
        try { msg = problemMessage(await versionRes.json(), fallback) } catch { /* body not JSON — keep default msg */ }
        throw { step: 'version', message: msg }
      }

      // Publish (reviewer) or submit for review (editor), if requested. The
      // agent + version exist either way, so a failure here is reported as a
      // warning on the detail page rather than aborting the create.
      let warning: string | undefined
      if (formData.get('publish') === 'on') {
        const action = canReview ? 'publish' : 'submit'
        const res = await authFetch(`/api/v1/agents/${ns}/${slug}/versions/${version}/${action}`, {
          method: 'POST',
        })
        if (!res.ok) {
          const fallback = `Version created, but ${action} failed (HTTP ${res.status}).`
          warning = fallback
          try { warning = `Version created, but ${action} failed: ${problemMessage(await res.json(), fallback)}` } catch { /* body not JSON — keep default msg */ }
        }
      }

      return { namespace: ns, slug, warning }
    },
    onSuccess: ({ namespace: ns, slug, warning }) => {
      // flushSync so the navigation blocker sees the form as clean before
      // the redirect below — otherwise the guard would block its own
      // success navigation.
      flushSync(() => setDirty(false))
      // Drop the cached admin list so the new agent appears immediately on
      // return; the 30s staleTime would otherwise hide it until a refetch.
      queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
      if (warning) toast.error(warning)
      else if (canReview) toast.success('Agent created')
      else toast.success('Agent created — version submitted for review')
      navigate(`/admin/agents/${ns}/${slug}`)
    },
    onError: (err: CreateError) => {
      setFormError(err)
    },
  })

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    mutation.mutate(new FormData(e.currentTarget))
  }

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/agents" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Agents
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Agent</span>
      </nav>

      <h1 className="text-2xl font-bold">New Agent</h1>

      {formError && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">
              {formError.step === 'version' ? 'Agent created, but version creation failed' : 'Failed to create agent'}
            </p>
            <p className="mt-0.5 text-destructive/80">{formError.message}</p>
          </div>
        </div>
      )}

      <DirtyFormGuard when={dirty} />

      <form onSubmit={handleSubmit} onChange={() => setDirty(true)} className="space-y-6">
        {/* ── Agent metadata ───────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Agent Details</CardTitle>
            <CardDescription>Basic metadata for the AI agent.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="namespace-select">
                Publisher <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Select
                value={namespace}
                // Ignore Radix's spurious empty-value callback (fired when the
                // controlled value has no mounted item yet) so it can't clobber
                // the pre-selected default; a real pick is always a slug.
                onValueChange={(v) => { if (v) { setPicked(v); setDirty(true) } }}
                required
              >
                <SelectTrigger id="namespace-select" aria-required="true">
                  <SelectValue placeholder="Select publisher…">
                    {selectedPublisher
                      ? `${selectedPublisher.slug} — ${selectedPublisher.name}`
                      : undefined}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {publishers.map((p) => (
                    <SelectItem key={p.slug} value={p.slug}>
                      {p.slug} — {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <SlugField placeholder="my-agent" />

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
              Leave &quot;Version&quot; blank to create the agent as a draft without a version.
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
              <Select value={authScheme} onValueChange={(v) => { setAuthScheme(v); setDirty(true) }}>
                <SelectTrigger id="auth-scheme-select">
                  <SelectValue placeholder="None / public" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_none">None / public</SelectItem>
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
                        defaultChecked={m.value === 'text/plain'}
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
                        defaultChecked={m.value === 'text/plain'}
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

            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <input
                  id="publish"
                  name="publish"
                  type="checkbox"
                  defaultChecked
                  className="h-4 w-4 rounded border border-input accent-primary"
                />
                <Label htmlFor="publish" className="cursor-pointer font-normal">
                  {canReview ? 'Publish version immediately' : 'Submit version for review'}
                </Label>
              </div>
              {!canReview && (
                <p className="text-xs text-muted-foreground pl-6">
                  A reviewer approves it before it goes live.
                </p>
              )}
            </div>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full" disabled={mutation.isPending || !namespace}>
          {mutation.isPending ? 'Creating…' : 'Create Agent'}
        </Button>
      </form>
    </div>
  )
}
