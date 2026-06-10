import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  ClipboardCheck,
  CheckCircle2,
  XCircle,
  GitPullRequestArrow,
  Trash2,
  Eye,
  Archive,
  Pencil,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Input } from '@/components/ui/input'
import { EmptyState } from '@/components/ui/empty-state'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import type { components } from '@/lib/schema'

type Item = components['schemas']['ReviewQueueItem']
type MCPVersion = components['schemas']['MCPServerVersion']
type AgentVersion = components['schemas']['AgentVersion']

// The discriminator values returned by the server. Keep in sync with the
// OpenAPI ReviewQueueItem.kind enum.
type Kind =
  | 'mcp_version'
  | 'agent_version'
  | 'mcp_deletion'
  | 'agent_deletion'
  | 'mcp_change'
  | 'agent_change'

function isVersion(kind: Kind): boolean {
  return kind === 'mcp_version' || kind === 'agent_version'
}

function isChange(kind: Kind): boolean {
  return kind === 'mcp_change' || kind === 'agent_change'
}

function isMCP(kind: Kind): boolean {
  return kind.startsWith('mcp')
}

// Friendly label for an entry-change action.
function actionLabel(action?: string): string {
  switch (action) {
    case 'visibility':
      return 'Visibility change'
    case 'deprecation':
      return 'Deprecation'
    case 'undeprecation':
      return 'Republish'
    case 'metadata_edit':
      return 'Metadata edit'
    default:
      return 'Entry change'
  }
}

function kindLabel(it: Item): string {
  const kind = it.kind as Kind
  switch (kind) {
    case 'mcp_version':
      return 'MCP version'
    case 'agent_version':
      return 'Agent version'
    case 'mcp_deletion':
      return 'MCP deletion'
    case 'agent_deletion':
      return 'Agent deletion'
    case 'mcp_change':
      return `MCP · ${actionLabel(it.action)}`
    case 'agent_change':
      return `Agent · ${actionLabel(it.action)}`
  }
}

function kindIcon(it: Item) {
  const kind = it.kind as Kind
  if (isVersion(kind)) return GitPullRequestArrow
  if (kind === 'mcp_deletion' || kind === 'agent_deletion') return Trash2
  switch (it.action) {
    case 'visibility':
      return Eye
    case 'deprecation':
    case 'undeprecation':
      return Archive
    default:
      return Pencil
  }
}

// Render the proposed mutation of an entry-change item so the reviewer sees
// exactly what they're approving.
function ChangeDetails({ it }: { it: Item }) {
  const payload = (it.payload ?? {}) as Record<string, unknown>
  if (it.action === 'visibility') {
    return (
      <p className="text-xs text-muted-foreground">
        Set visibility to{' '}
        <span className="font-mono">{String(payload.visibility ?? '?')}</span>
      </p>
    )
  }
  if (it.action === 'deprecation') {
    return (
      <p className="text-xs text-muted-foreground">
        Mark this entry as <span className="font-mono">deprecated</span>
      </p>
    )
  }
  if (it.action === 'undeprecation') {
    return (
      <p className="text-xs text-muted-foreground">
        Republish this deprecated entry (back to{' '}
        <span className="font-mono">published</span>)
      </p>
    )
  }
  // metadata_edit: list the proposed fields.
  const entries = Object.entries(payload).filter(([, v]) => v !== '' && v != null)
  if (entries.length === 0) return null
  return (
    <dl className="text-xs text-muted-foreground grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5">
      {entries.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="font-mono">{k}</dt>
          <dd className="font-mono truncate">{String(v)}</dd>
        </div>
      ))}
    </dl>
  )
}

// VersionContent — expandable panel showing what a version submission
// actually contains, fetched lazily from the existing version GET endpoints.
// Reviewers previously had to open the entry and read the raw API response to
// see the content they were approving (J2).
function VersionContent({ it }: { it: Item }) {
  const api = useAuthClient()
  const [expanded, setExpanded] = useState(false)
  const mcp = isMCP(it.kind as Kind)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['review-version-content', it.kind, it.publisher_slug, it.entry_slug, it.version],
    queryFn: async () => {
      const path = { namespace: it.publisher_slug, slug: it.entry_slug, version: it.version! }
      if (mcp) {
        const r = await api.GET('/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}', { params: { path } })
        return { mcp: r.data ?? null, agent: null }
      }
      const r = await api.GET('/api/v1/agents/{namespace}/{slug}/versions/{version}', { params: { path } })
      return { mcp: null, agent: r.data ?? null }
    },
    enabled: expanded,
    staleTime: 30_000,
  })

  const Chevron = expanded ? ChevronDown : ChevronRight
  return (
    <div>
      <button
        type="button"
        className="flex items-center gap-1 text-xs text-primary hover:underline"
        aria-expanded={expanded}
        onClick={() => setExpanded((v) => !v)}
      >
        <Chevron className="h-3.5 w-3.5" aria-hidden="true" />
        {expanded ? 'Hide submitted content' : 'Show submitted content'}
      </button>
      {expanded && (
        <div className="mt-2 rounded-md border bg-muted/30 p-3">
          {isLoading ? (
            <Skeleton className="h-12 w-full rounded" />
          ) : isError || !data ? (
            <p className="text-xs text-destructive">Failed to load the version's content.</p>
          ) : data.mcp ? (
            <MCPVersionSummary v={data.mcp} />
          ) : data.agent ? (
            <AgentVersionSummary v={data.agent} />
          ) : (
            <p className="text-xs text-muted-foreground">No content found.</p>
          )}
        </div>
      )}
    </div>
  )
}

function SummaryRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="contents">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-mono break-words">{children}</dd>
    </div>
  )
}

function MCPVersionSummary({ v }: { v: MCPVersion }) {
  const tools = v.tools ?? []
  const packages = v.packages ?? []
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-xs">
      <SummaryRow label="Runtime">{v.runtime}</SummaryRow>
      <SummaryRow label="Protocol">{v.protocol_version}</SummaryRow>
      <SummaryRow label="Packages">
        {packages.length === 0
          ? '—'
          : packages.map((p) => `${p.registryType}: ${p.identifier}@${p.version} (${p.transport.type})`).join(', ')}
      </SummaryRow>
      <SummaryRow label="Tools">
        {tools.length === 0 ? '—' : tools.map((t) => t.name).join(', ')}
      </SummaryRow>
    </dl>
  )
}

function AgentVersionSummary({ v }: { v: AgentVersion }) {
  const skills = v.skills ?? []
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-xs">
      <SummaryRow label="Endpoint">{v.endpoint_url}</SummaryRow>
      <SummaryRow label="Protocol">{v.protocol_version}</SummaryRow>
      <SummaryRow label="Skills">
        {skills.length === 0 ? '—' : skills.map((s) => s.name).join(', ')}
      </SummaryRow>
      <SummaryRow label="Auth">
        {(v.authentication ?? []).length === 0 ? '—' : (v.authentication ?? []).map((a) => a.scheme).join(', ')}
      </SummaryRow>
    </dl>
  )
}

// 409 problem-type suffix → friendly message. Anything we don't recognise
// falls through to the generic detail field.
function friendlyProblem(type?: string, detail?: string): string {
  if (!type) return detail ?? 'Action failed'
  if (type.endsWith('review-revision-mismatch'))
    return 'The item was edited since this queue page loaded. Refresh to review the latest revision.'
  if (type.endsWith('review-state-mismatch'))
    return 'The item is no longer pending review (already approved, rejected, or withdrawn).'
  if (type.endsWith('already-published'))
    return 'The version is already published.'
  if (type.endsWith('review-already-pending'))
    return 'Another item on this entry is already pending review.'
  if (type.endsWith('change-already-pending'))
    return 'Another change on this entry is already pending review.'
  if (type.endsWith('change-not-applicable'))
    return 'The entry is no longer in a state where this change can be applied. Reject this request.'
  return detail ?? 'Action failed'
}

export default function AdminReviewQueue() {
  const api = useAuthClient()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState<string | null>(null)
  // Per-row reject form state. Keyed by a stable identifier built from the
  // item's URL parameters so two open forms don't collide.
  const [rejectingKey, setRejectingKey] = useState<string | null>(null)
  const [rejectReason, setRejectReason] = useState<string>('')
  // Item whose approval awaits confirmation. Approve applies/publishes (or
  // hard-deletes, for deletion requests) immediately — it must not be a
  // single unconfirmed click (J2).
  const [confirmTarget, setConfirmTarget] = useState<Item | null>(null)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['admin-review-queue'],
    queryFn: () =>
      api.GET('/api/v1/review-queue').then((r) => r.data),
    enabled: true,
  })

  const items = (data?.items ?? []) as Item[]

  function rowKey(it: Item): string {
    // Versions are uniquely identified by entry + version; deletions by entry
    // alone; changes by entry + action. Suffix the kind + action so two items
    // on the same entry never collide.
    return [
      it.kind,
      it.publisher_slug,
      it.entry_slug,
      it.version ?? '',
      it.action ?? '',
    ].join('|')
  }

  // The URL builders below correspond to the endpoint families exposed by the
  // workflow. They return the path + body shape; the mutation wires them
  // through openapi-fetch.
  async function approveItem(it: Item) {
    const ns = it.publisher_slug
    const slug = it.entry_slug
    const rev = it.revision!
    switch (it.kind as Kind) {
      case 'mcp_version':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/approve',
          { params: { path: { namespace: ns, slug, version: it.version! } }, body: { revision: rev } },
        )
      case 'agent_version':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/versions/{version}/approve',
          { params: { path: { namespace: ns, slug, version: it.version! } }, body: { revision: rev } },
        )
      case 'mcp_deletion':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request/approve',
          { params: { path: { namespace: ns, slug } } },
        )
      case 'agent_deletion':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/deletion-request/approve',
          { params: { path: { namespace: ns, slug } } },
        )
      case 'mcp_change':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/change-request/approve',
          { params: { path: { namespace: ns, slug } }, body: { revision: rev } },
        )
      case 'agent_change':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/change-request/approve',
          { params: { path: { namespace: ns, slug } }, body: { revision: rev } },
        )
    }
  }

  async function rejectItem(it: Item, reason: string) {
    const ns = it.publisher_slug
    const slug = it.entry_slug
    const rev = it.revision!
    switch (it.kind as Kind) {
      case 'mcp_version':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/reject',
          { params: { path: { namespace: ns, slug, version: it.version! } }, body: { revision: rev, reason } },
        )
      case 'agent_version':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/versions/{version}/reject',
          { params: { path: { namespace: ns, slug, version: it.version! } }, body: { revision: rev, reason } },
        )
      case 'mcp_deletion':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request/reject',
          { params: { path: { namespace: ns, slug } }, body: { reason } },
        )
      case 'agent_deletion':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/deletion-request/reject',
          { params: { path: { namespace: ns, slug } }, body: { reason } },
        )
      case 'mcp_change':
        return api.POST(
          '/api/v1/mcp/servers/{namespace}/{slug}/change-request/reject',
          { params: { path: { namespace: ns, slug } }, body: { revision: rev, reason } },
        )
      case 'agent_change':
        return api.POST(
          '/api/v1/agents/{namespace}/{slug}/change-request/reject',
          { params: { path: { namespace: ns, slug } }, body: { revision: rev, reason } },
        )
    }
  }

  function approveToast(it: Item): string {
    const ref = `${it.publisher_slug}/${it.entry_slug}`
    const kind = it.kind as Kind
    if (isVersion(kind)) return `Approved ${ref} v${it.version}`
    if (isChange(kind)) return `Applied ${actionLabel(it.action).toLowerCase()} on ${ref}`
    return `Confirmed deletion of ${ref}`
  }

  function rejectToast(it: Item): string {
    const ref = `${it.publisher_slug}/${it.entry_slug}`
    const kind = it.kind as Kind
    if (isVersion(kind)) return `Rejected ${ref} v${it.version}`
    if (isChange(kind)) return `Rejected ${actionLabel(it.action).toLowerCase()} on ${ref}`
    return `Cancelled deletion of ${ref}`
  }

  const approveMutation = useMutation({
    mutationFn: async (it: Item) => {
      setActionError(null)
      const res = await approveItem(it)
      if (res?.error) {
        const e = res.error as { type?: string; detail?: string }
        throw new Error(friendlyProblem(e.type, e.detail))
      }
      return it
    },
    onSuccess: (it: Item) => {
      toast.success(approveToast(it))
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue'] })
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const rejectMutation = useMutation({
    mutationFn: async ({ item, reason }: { item: Item; reason: string }) => {
      setActionError(null)
      const res = await rejectItem(item, reason)
      if (res?.error) {
        const e = res.error as { type?: string; detail?: string }
        throw new Error(friendlyProblem(e.type, e.detail))
      }
      return item
    },
    onSuccess: (item: Item) => {
      toast.success(rejectToast(item))
      setRejectingKey(null)
      setRejectReason('')
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue'] })
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <ClipboardCheck className="h-6 w-6 text-muted-foreground" aria-hidden="true" />
        <h1 className="text-2xl font-bold">Review queue</h1>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={() => refetch()}
          disabled={isLoading}
        >
          Refresh
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Pending versions, entry changes (visibility, deprecation, metadata), and
        deletion requests across the registry, newest first. Approve applies the
        change; reject records the supplied reason.
      </p>

      {actionError && (
        <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {actionError}
        </div>
      )}

      {isLoading ? (
        <ul className="divide-y rounded-md border" aria-busy="true" aria-label="Loading queue">
          {Array.from({ length: 4 }).map((_, i) => (
            <li key={i} className="p-4 space-y-2">
              <div className="flex items-center gap-3">
                <Skeleton className="h-5 w-28 rounded" />
                <Skeleton className="h-4 w-40 rounded" />
                <Skeleton className="h-5 w-14 rounded ml-auto" />
              </div>
              <Skeleton className="h-3 w-56 rounded" />
              <div className="flex gap-2 pt-1">
                <Skeleton className="h-8 w-24 rounded" />
                <Skeleton className="h-8 w-24 rounded" />
              </div>
            </li>
          ))}
        </ul>
      ) : isError ? (
        <p className="text-sm text-destructive">Failed to load review queue.</p>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<ClipboardCheck className="h-10 w-10" />}
          title="Queue is empty"
          description="Nothing is currently pending review. Submitted versions, entry changes, and deletion requests will show up here."
        />
      ) : (
        <ul className="divide-y rounded-md border">
          {items.map((it) => {
            const kind = it.kind as Kind
            const Icon = kindIcon(it)
            const key = rowKey(it)
            const isRejecting = rejectingKey === key
            const detailHref = `/admin/${isMCP(kind) ? 'mcp' : 'agents'}/${it.publisher_slug}/${it.entry_slug}`
            return (
              <li key={key} className="p-4 space-y-2">
                <div className="flex items-start gap-3 flex-wrap">
                  <Badge variant="outline" className="text-xs flex items-center gap-1.5">
                    <Icon className="h-3 w-3" />
                    {kindLabel(it)}
                  </Badge>
                  <Link
                    to={detailHref}
                    className="text-sm font-mono text-primary hover:underline"
                  >
                    {it.publisher_slug}/{it.entry_slug}
                  </Link>
                  {it.version && (
                    <Badge variant="secondary" className="text-xs">
                      v{it.version}
                    </Badge>
                  )}
                  {typeof it.revision === 'number' && (
                    <Badge variant="outline" className="text-xs">
                      rev {it.revision}
                    </Badge>
                  )}
                  {it.request_public && (
                    <Badge
                      variant="default"
                      className="text-xs"
                      title="The submitter asked for this entry to be made public when the version is approved"
                    >
                      public on approval
                    </Badge>
                  )}
                  <span className="text-xs text-muted-foreground ml-auto">
                    {formatDate(it.submitted_at)}
                  </span>
                </div>
                {isChange(kind) && <ChangeDetails it={it} />}
                {isVersion(kind) && it.version && <VersionContent it={it} />}
                {it.submitted_by_email && (
                  <p className="text-xs text-muted-foreground">
                    Submitted by{' '}
                    <span className="font-mono">{it.submitted_by_email}</span>
                  </p>
                )}

                <div className="flex flex-wrap items-center gap-2 pt-1">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setConfirmTarget(it)}
                    disabled={approveMutation.isPending || rejectMutation.isPending}
                  >
                    <CheckCircle2 className="h-4 w-4" />
                    <span className="ml-1.5">Approve</span>
                  </Button>
                  {!isRejecting && (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setRejectingKey(key)
                        setRejectReason('')
                        setActionError(null)
                      }}
                      disabled={approveMutation.isPending || rejectMutation.isPending}
                    >
                      <XCircle className="h-4 w-4" />
                      <span className="ml-1.5">Reject</span>
                    </Button>
                  )}
                </div>

                {isRejecting && (
                  <form
                    className="flex flex-wrap items-center gap-2 pt-1"
                    onSubmit={(e) => {
                      e.preventDefault()
                      if (rejectReason.trim() === '') return
                      rejectMutation.mutate({ item: it, reason: rejectReason.trim() })
                    }}
                  >
                    <Input
                      autoFocus
                      placeholder="Reason (required)"
                      value={rejectReason}
                      onChange={(e) => setRejectReason(e.target.value)}
                      className="max-w-md"
                    />
                    <Button
                      size="sm"
                      type="submit"
                      disabled={rejectReason.trim() === '' || rejectMutation.isPending}
                    >
                      Confirm reject
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      type="button"
                      onClick={() => {
                        setRejectingKey(null)
                        setRejectReason('')
                      }}
                      disabled={rejectMutation.isPending}
                    >
                      Cancel
                    </Button>
                  </form>
                )}
              </li>
            )
          })}
        </ul>
      )}

      {confirmTarget && (() => {
        const it = confirmTarget
        const ref = `${it.publisher_slug}/${it.entry_slug}`
        const kind = it.kind as Kind
        const isDeletion = kind === 'mcp_deletion' || kind === 'agent_deletion'
        const title = isVersion(kind)
          ? `Approve ${ref} v${it.version}?`
          : isDeletion
            ? `Approve deletion of ${ref}?`
            : `Approve ${actionLabel(it.action).toLowerCase()} on ${ref}?`
        // A version approval that also flips visibility is a bigger decision
        // than a plain publish — the confirm must say so (author-declared
        // request_public).
        const description = isVersion(kind)
          ? it.request_public
            ? 'Approval publishes this version AND makes the entry public — it becomes visible to everyone immediately.'
            : 'Approval publishes this version to the registry immediately.'
          : isDeletion
            ? 'This permanently deletes the entry and all its versions. It disappears from the registry immediately.'
            : 'The change is applied to the entry immediately.'
        return (
          <ConfirmDialog
            open
            onOpenChange={(o) => { if (!o) setConfirmTarget(null) }}
            title={title}
            description={description}
            confirmLabel={isVersion(kind) ? 'Approve & publish' : isDeletion ? 'Approve deletion' : 'Approve change'}
            destructive={isDeletion}
            isPending={approveMutation.isPending}
            onConfirm={() => {
              approveMutation.mutate(it)
              setConfirmTarget(null)
            }}
          />
        )
      })()}
    </div>
  )
}
