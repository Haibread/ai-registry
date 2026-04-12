import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
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
import { useAuth } from '@/auth/AuthContext'

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
  const { accessToken, clearSession } = useAuth()
  const navigate = useNavigate()

  const [namespace, setNamespace] = useState('')
  const [authScheme, setAuthScheme] = useState('_none')
  const [formError, setFormError] = useState<CreateError | null>(null)

  const api = useAuthClient()

  const { data: publishersData } = useQuery({
    queryKey: ['publishers'],
    queryFn: () => api.GET('/api/v1/publishers', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: !!accessToken,
  })

  const publishers = publishersData?.items ?? []

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
        const msg = (agentError as { title?: string } | undefined)?.title ?? 'Failed to create agent.'
        throw { step: undefined, message: msg }
      }

      // Step 2: Create version (optional)
      const version = (formData.get('version') as string).trim()
      const endpointUrl = (formData.get('endpoint_url') as string).trim()
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

      const versionRes = await fetch(`/api/v1/agents/${ns}/${slug}/versions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken ?? ''}`,
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
        if (versionRes.status === 401) { await clearSession(); return { namespace: ns, slug } }
        let msg = `Failed to create version (HTTP ${versionRes.status}).`
        try { const body = await versionRes.json(); if (body?.title) msg = body.title } catch {}
        throw { step: 'version', message: msg }
      }

      if (formData.get('publish') === 'on') {
        const publishRes = await fetch(`/api/v1/agents/${ns}/${slug}/versions/${version}/publish`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${accessToken ?? ''}` },
        })
        if (publishRes.status === 401) { await clearSession(); return { namespace: ns, slug } }
      }

      return { namespace: ns, slug }
    },
    onSuccess: ({ namespace: ns, slug }) => {
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
    <div className="space-y-6 max-w-lg mx-auto">
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

      <form onSubmit={handleSubmit} className="space-y-6">
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
              <Select value={authScheme} onValueChange={setAuthScheme}>
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

        <Button type="submit" className="w-full" disabled={mutation.isPending || !namespace}>
          {mutation.isPending ? 'Creating…' : 'Create Agent'}
        </Button>
      </form>
    </div>
  )
}
