import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { ArrowLeft, Cpu, Shield, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { LifecycleStepper } from '@/components/admin/lifecycle-stepper'
import { DeprecateButton } from '@/components/admin/deprecate-button'
import { DeleteButton } from '@/components/admin/delete-button'
import { RequestDeletionButton } from '@/components/admin/request-deletion-button'
import { VersionsSection } from '@/components/admin/versions-section'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { ErrorState } from '@/components/ui/error-state'
import { useAuthClient } from '@/lib/api-client'
import { formatDate, problemMessage, HTTPError, isNotFound } from '@/lib/utils'
import { usePermissions } from '@/auth/useMe'
import type { components } from '@/lib/schema'

type AgentSkill = components['schemas']['AgentSkill']

export default function AdminAgentDetail() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const perms = usePermissions()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const api = useAuthClient()
  const { data, isPending, isError, error, refetch } = useQuery({
    queryKey: ['admin-agent-detail', ns, slug],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      // Carry the HTTP status so the error branch can tell a real 404 from
      // a server error or network failure (P2.6).
      if (error || !data) throw new HTTPError(problemMessage(error, 'Failed to load this agent.'), response?.status)
      return data
    },
    enabled: !!ns && !!slug && true,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-agent-detail', ns, slug] })
    queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
    queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
  }

  // Editors enqueue these actions for review; Server Admins apply immediately.
  const submitToast = (appliedMsg: string) =>
    perms.isServerAdmin ? appliedMsg : 'Submitted for review'

  const visibilityMutation = useMutation({
    mutationFn: async (newVisibility: 'public' | 'private') => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/visibility', {
        params: { path: { namespace: ns!, slug: slug! } },
        body: { visibility: newVisibility },
      })
      if (error) throw new Error(problemMessage(error, 'Failed to change visibility.'))
      return newVisibility
    },
    onSuccess: (v) => { invalidate(); toast.success(submitToast(`Now ${v}`)) },
    onError: (e: Error) => toast.error(e.message),
  })

  const deprecateMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/deprecate', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      if (error) throw new Error(problemMessage(error, 'Failed to deprecate.'))
    },
    onSuccess: () => { invalidate(); toast.success(submitToast('Agent deprecated')) },
    onError: (e: Error) => toast.error(e.message),
  })

  const undeprecateMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/undeprecate', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      if (error) throw new Error(problemMessage(error, 'Failed to republish.'))
    },
    onSuccess: () => { invalidate(); toast.success(submitToast('Agent republished')) },
    onError: (e: Error) => toast.error(e.message),
  })

  const editMutation = useMutation({
    mutationFn: async (body: { name: string; description: string }) => {
      const { error } = await api.PATCH('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: ns!, slug: slug! } },
        body,
      })
      if (error) throw new Error(problemMessage(error, 'Update failed.'))
    },
    onSuccess: () => { invalidate(); setEditOpen(false); toast.success(submitToast('Agent updated')) },
  })

  const withdrawChangeMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/change-request/withdraw', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      if (error) throw new Error(problemMessage(error, 'Failed to withdraw change.'))
    },
    onSuccess: () => { invalidate(); toast.success('Change withdrawn') },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.DELETE('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      if (error) throw new Error(problemMessage(error, 'Delete failed.'))
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ['admin-agents'] })
      toast.success('Agent deleted')
      navigate('/admin/agents')
    },
    onError: (e: Error) => toast.error(e.message),
  })

  if (isPending) return <DetailPageSkeleton />
  if (isError || !data) return (
    <div className="space-y-4 max-w-3xl mx-auto">
      {isNotFound(error) ? (
        <p className="text-destructive">Not found — this agent may have been deleted or renamed.</p>
      ) : (
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load this agent.'}
          onRetry={() => refetch()}
        />
      )}
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/agents')}>Back to Agents</Button>
    </div>
  )

  const lv = data.latest_version
  const pendingChange = data.pending_change
  const changePending = !!pendingChange && !perms.isServerAdmin
  const changeActionLabel: Record<string, string> = {
    visibility: 'Visibility change',
    deprecation: 'Deprecation',
    undeprecation: 'Republish',
    metadata_edit: 'Metadata edit',
  }

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/agents" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Agents
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{data.namespace}/{data.slug}</span>
      </nav>

      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
        <div className="flex gap-2">
          {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
          <StatusBadge status={data.status} />
          <VisibilityBadge visibility={data.visibility} />
        </div>
      </div>

      <LifecycleStepper
        currentStatus={data.status}
        // Read-only for viewers; no clickable targets while a change is pending.
        // For published entries the stepper is informational only — deprecation
        // belongs to the Actions row's Deprecate button, which confirms first
        // (one transition surface per action).
        allowedTransitions={
          !perms.canEdit(ns) || changePending || data.status === 'published' ? [] : undefined
        }
        onTransition={(target) => {
          if (target === 'published' && data.status === 'deprecated') undeprecateMutation.mutate()
          else if (target === 'published' && data.status === 'draft') {
            // Publishing happens per version — point at the Versions section
            // instead of silently dropping the click.
            document.getElementById('versions-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
            toast.info('Publishing happens per version — submit or publish a version below.')
          }
        }}
      />

      {/* Editors get the pipeline spelled out once: nothing else in the UI
          explains that publish and make-public are separate reviewed steps (J1). */}
      {perms.canEdit(ns) && !perms.isServerAdmin && data.visibility === 'private' && (
        <p className="text-sm text-muted-foreground max-w-prose">
          How this goes live: author a version, submit it for review, a
          reviewer approves it — then &ldquo;Make public&rdquo; (also reviewed)
          exposes the entry in the public registry.
        </p>
      )}

      {pendingChange && (
        <div
          role="status"
          className="flex flex-wrap items-center gap-3 rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm"
        >
          <span>
            <span className="font-medium">
              {changeActionLabel[pendingChange.action] ?? 'Change'}
            </span>{' '}
            pending review
            {pendingChange.submitted_by_email && (
              <> — submitted by <span className="font-mono">{pendingChange.submitted_by_email}</span></>
            )}
          </span>
          {perms.canEdit(ns) && (
            <Button
              variant="outline"
              size="sm"
              className="ml-auto"
              disabled={withdrawChangeMutation.isPending}
              onClick={() => withdrawChangeMutation.mutate()}
            >
              Withdraw
            </Button>
          )}
        </div>
      )}

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
        <dt className="text-muted-foreground">Namespace / Slug</dt>
        <dd className="font-mono">{data.namespace}/{data.slug}</dd>
        {/* Every editable field renders, with an explicit "—" for unset
            values, so a reader can tell "not set" from "not shown" (P2.3). */}
        <dt className="text-muted-foreground">Description</dt>
        <dd className="max-w-prose">{data.description || <span className="text-muted-foreground">—</span>}</dd>
        {lv && (
          <>
            {lv.endpoint_url && (
              <>
                <dt className="text-muted-foreground">Endpoint</dt>
                <dd>
                  <a
                    href={lv.endpoint_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-mono text-xs hover:underline break-all"
                  >
                    {lv.endpoint_url}
                  </a>
                </dd>
              </>
            )}
            {lv.protocol_version && (
              <>
                <dt className="text-muted-foreground">A2A protocol</dt>
                <dd className="font-mono">{lv.protocol_version}</dd>
              </>
            )}
            {lv.published_at && (
              <>
                <dt className="text-muted-foreground">Published</dt>
                <dd>{formatDate(lv.published_at)}</dd>
              </>
            )}
            {lv.default_input_modes && lv.default_input_modes.length > 0 && (
              <>
                <dt className="text-muted-foreground">Input modes</dt>
                <dd className="flex flex-wrap gap-1">
                  {lv.default_input_modes.map((m) => (
                    <Badge key={m} variant="secondary" className="text-xs">{m}</Badge>
                  ))}
                </dd>
              </>
            )}
            {lv.default_output_modes && lv.default_output_modes.length > 0 && (
              <>
                <dt className="text-muted-foreground">Output modes</dt>
                <dd className="flex flex-wrap gap-1">
                  {lv.default_output_modes.map((m) => (
                    <Badge key={m} variant="secondary" className="text-xs">{m}</Badge>
                  ))}
                </dd>
              </>
            )}
            {lv.authentication && lv.authentication.length > 0 && (
              <>
                <dt className="text-muted-foreground flex items-center gap-1">
                  <Shield className="h-3.5 w-3.5" /> Auth schemes
                </dt>
                <dd className="flex flex-wrap gap-1">
                  {lv.authentication.map((scheme, i) => {
                    const s = scheme as Record<string, string>
                    const label = s['scheme'] ?? s['type'] ?? `scheme ${i + 1}`
                    return (
                      <Badge key={i} variant="outline" className="text-xs">{label}</Badge>
                    )
                  })}
                </dd>
              </>
            )}
          </>
        )}
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(data.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(data.updated_at)}</dd>
      </dl>

      {/* Skills grid */}
      {lv?.skills && lv.skills.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Cpu className="h-4 w-4" aria-hidden="true" /> Skills
          </h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {lv.skills.map((skill: AgentSkill) => (
              <Card key={skill.id} className="bg-muted/30">
                <CardHeader className="pb-2 pt-4 px-4">
                  <CardTitle className="text-sm">{skill.name}</CardTitle>
                  <CardDescription className="text-xs">{skill.description}</CardDescription>
                </CardHeader>
                {(skill.tags.length > 0 || (skill.examples && skill.examples.length > 0)) && (
                  <CardContent className="pb-3 px-4 space-y-2">
                    {skill.tags.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {skill.tags.map((tag) => (
                          <Badge key={tag} variant="secondary" className="text-[10px] px-1.5 py-0">
                            {tag}
                          </Badge>
                        ))}
                      </div>
                    )}
                    {skill.examples && skill.examples.length > 0 && (
                      <div className="space-y-1">
                        <p className="text-[10px] text-muted-foreground uppercase tracking-wide">Examples</p>
                        <ul className="text-xs space-y-0.5 text-muted-foreground">
                          {skill.examples.slice(0, 3).map((ex, i) => (
                            <li key={i} className="truncate">• {ex}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </CardContent>
                )}
              </Card>
            ))}
          </div>
        </div>
      )}

      <Separator />

      {/* Edit form */}
      {editOpen && (
        <form
          className="space-y-4 border rounded-lg p-4"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            editMutation.mutate({
              name: fd.get('name') as string,
              description: fd.get('description') as string,
            })
          }}
        >
          <h2 className="text-lg font-semibold">Edit Agent</h2>
          <div className="grid gap-3">
            <div className="space-y-1">
              <Label htmlFor="name">Name <span className="text-destructive">*</span></Label>
              <Input id="name" name="name" defaultValue={data.name} required />
            </div>
            <div className="space-y-1">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" defaultValue={data.description ?? ''} />
            </div>
          </div>
          {editMutation.isError && (
            <p role="alert" className="text-sm text-destructive">
              {editMutation.error.message || 'Update failed. Please try again.'}
            </p>
          )}
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={editMutation.isPending}>
              {editMutation.isPending ? 'Saving…' : 'Save changes'}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}

      {/* Role-gated actions: edit/deprecate/request-deletion need
          Editor on this publisher; visibility + direct delete are Server-Admin
          only. The server still enforces each. */}
      {(perms.canEdit(ns) || perms.isServerAdmin) && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold">Actions</h2>
          <div className="flex flex-wrap gap-2">
            {perms.canEdit(ns) && (
              <Button
                variant="outline"
                size="sm"
                disabled={changePending && !editOpen}
                title={changePending ? 'A change is already pending review' : undefined}
                onClick={() => setEditOpen(v => !v)}
              >
                {editOpen ? 'Cancel edit' : 'Edit'}
              </Button>
            )}

            {perms.canEdit(ns) && (() => {
              // Public requires an approved (published) version; private is
              // always allowed. Mirrors the server precondition.
              const wantsPublic = data.visibility !== 'public'
              const isApproved = data.status === 'published' || data.status === 'deprecated'
              const blocked = wantsPublic && !isApproved
              return (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={visibilityMutation.isPending || blocked || changePending}
                  title={
                    changePending
                      ? 'A change is already pending review'
                      : blocked
                        ? 'Approve a published version before making this public'
                        : undefined
                  }
                  onClick={() => visibilityMutation.mutate(data.visibility === 'public' ? 'private' : 'public')}
                >
                  Make {data.visibility === 'public' ? 'private' : 'public'}
                </Button>
              )
            })()}

            {perms.canEdit(ns) && data.status === 'published' && !changePending && (
              <DeprecateButton
                onDeprecate={() => deprecateMutation.mutate()}
                entityName={data.name}
              />
            )}

            {perms.canEdit(ns) && data.status === 'deprecated' && !changePending && (
              <Button
                variant="outline"
                size="sm"
                disabled={undeprecateMutation.isPending}
                onClick={() => undeprecateMutation.mutate()}
              >
                Republish
              </Button>
            )}

            {perms.canEdit(ns) && (
              <RequestDeletionButton
                kind="agent"
                namespace={data.namespace}
                slug={data.slug}
                entityName={data.name}
              />
            )}
            {perms.isServerAdmin && (
              <DeleteButton
                onDelete={() => deleteMutation.mutate()}
                entityName={data.name}
                isPending={deleteMutation.isPending}
              />
            )}
          </div>
        </div>
      )}

      <Separator />

      <div className="space-y-2">
        <h2 className="text-lg font-semibold">A2A Agent Card</h2>
        <p className="text-sm text-muted-foreground">
          Published at the well-known path for A2A discovery.
        </p>
        <Button variant="outline" size="sm" asChild>
          <a
            href={`/agents/${ns}/${slug}/.well-known/agent-card.json`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5"
          >
            <ExternalLink className="h-4 w-4" /> View agent card
          </a>
        </Button>
      </div>

      <Separator />

      <div id="versions-section">
        <VersionsSection kind="agent" namespace={data.namespace} slug={data.slug} entryStatus={data.status} />
      </div>

      <Separator />

      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
