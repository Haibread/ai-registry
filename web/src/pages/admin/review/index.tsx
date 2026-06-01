import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  ClipboardCheck,
  CheckCircle2,
  XCircle,
  GitPullRequestArrow,
  Trash2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { EmptyState } from '@/components/ui/empty-state'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import type { components } from '@/lib/schema'

type Item = components['schemas']['ReviewQueueItem']

// The four discriminator values returned by the server. Keep in sync with
// the OpenAPI ReviewQueueItem.kind enum.
type Kind = 'mcp_version' | 'agent_version' | 'mcp_deletion' | 'agent_deletion'

function isVersion(kind: Kind): boolean {
  return kind === 'mcp_version' || kind === 'agent_version'
}

function kindLabel(kind: Kind): string {
  switch (kind) {
    case 'mcp_version':
      return 'MCP version'
    case 'agent_version':
      return 'Agent version'
    case 'mcp_deletion':
      return 'MCP deletion'
    case 'agent_deletion':
      return 'Agent deletion'
  }
}

function kindIcon(kind: Kind) {
  return isVersion(kind) ? GitPullRequestArrow : Trash2
}

// 409 problem-type suffix → friendly message. Anything we don't recognise
// falls through to the generic detail field.
function friendlyProblem(type?: string, detail?: string): string {
  if (!type) return detail ?? 'Action failed'
  if (type.endsWith('review-revision-mismatch'))
    return 'The version was edited since this queue page loaded. Refresh to review the latest revision.'
  if (type.endsWith('review-state-mismatch'))
    return 'The version is no longer pending review (already approved, rejected, or withdrawn).'
  if (type.endsWith('already-published'))
    return 'The version is already published.'
  if (type.endsWith('review-already-pending'))
    return 'Another item on this entry is already pending review.'
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

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['admin-review-queue'],
    queryFn: () =>
      api.GET('/api/v1/review-queue').then((r) => r.data),
    enabled: true,
  })

  const items = (data?.items ?? []) as Item[]

  function rowKey(it: Item): string {
    // Versions are uniquely identified by entry + version; deletions by
    // entry alone. Suffix the kind so a version and a deletion on the
    // same entry never collide.
    return [
      it.kind,
      it.publisher_slug,
      it.entry_slug,
      it.version ?? '',
    ].join('|')
  }

  // The four URL builders below correspond to the four endpoint families
  // exposed by the workflow. They return the path + the body shape; the
  // mutation wires them through openapi-fetch.
  async function approveItem(it: Item) {
    const ns = it.publisher_slug
    const slug = it.entry_slug
    const ver = it.version!
    const rev = it.revision!
    if (it.kind === 'mcp_version') {
      return api.POST(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/approve',
        { params: { path: { namespace: ns, slug, version: ver } }, body: { revision: rev } },
      )
    }
    if (it.kind === 'agent_version') {
      return api.POST(
        '/api/v1/agents/{namespace}/{slug}/versions/{version}/approve',
        { params: { path: { namespace: ns, slug, version: ver } }, body: { revision: rev } },
      )
    }
    if (it.kind === 'mcp_deletion') {
      return api.POST(
        '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request/approve',
        { params: { path: { namespace: ns, slug } } },
      )
    }
    return api.POST(
      '/api/v1/agents/{namespace}/{slug}/deletion-request/approve',
      { params: { path: { namespace: ns, slug } } },
    )
  }

  async function rejectItem(it: Item, reason: string) {
    const ns = it.publisher_slug
    const slug = it.entry_slug
    if (it.kind === 'mcp_version') {
      return api.POST(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/reject',
        {
          params: { path: { namespace: ns, slug, version: it.version! } },
          body: { revision: it.revision!, reason },
        },
      )
    }
    if (it.kind === 'agent_version') {
      return api.POST(
        '/api/v1/agents/{namespace}/{slug}/versions/{version}/reject',
        {
          params: { path: { namespace: ns, slug, version: it.version! } },
          body: { revision: it.revision!, reason },
        },
      )
    }
    if (it.kind === 'mcp_deletion') {
      return api.POST(
        '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request/reject',
        { params: { path: { namespace: ns, slug } }, body: { reason } },
      )
    }
    return api.POST(
      '/api/v1/agents/{namespace}/{slug}/deletion-request/reject',
      { params: { path: { namespace: ns, slug } }, body: { reason } },
    )
  }

  const approveMutation = useMutation({
    mutationFn: async (it: Item) => {
      setActionError(null)
      const { error } = await approveItem(it)
      if (error) {
        const e = error as { type?: string; detail?: string }
        throw new Error(friendlyProblem(e.type, e.detail))
      }
      return it
    },
    onSuccess: (it: Item) => {
      const isVer = isVersion(it.kind as Kind)
      toast.success(
        isVer
          ? `Approved ${it.publisher_slug}/${it.entry_slug} v${it.version}`
          : `Confirmed deletion of ${it.publisher_slug}/${it.entry_slug}`,
      )
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue'] })
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const rejectMutation = useMutation({
    mutationFn: async ({ item, reason }: { item: Item; reason: string }) => {
      setActionError(null)
      const { error } = await rejectItem(item, reason)
      if (error) {
        const e = error as { type?: string; detail?: string }
        throw new Error(friendlyProblem(e.type, e.detail))
      }
      return item
    },
    onSuccess: (item: Item) => {
      const isVer = isVersion(item.kind as Kind)
      toast.success(
        isVer
          ? `Rejected ${item.publisher_slug}/${item.entry_slug} v${item.version}`
          : `Cancelled deletion of ${item.publisher_slug}/${item.entry_slug}`,
      )
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
        Pending change-approval entries and deletion requests across the
        registry, newest first. Approve sends the item live; reject returns
        it to draft (with the supplied reason recorded on the version).
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
          description="Nothing is currently pending review. Submitted versions and deletion requests will show up here."
        />
      ) : (
        <ul className="divide-y rounded-md border">
          {items.map((it) => {
            const kind = it.kind as Kind
            const Icon = kindIcon(kind)
            const key = rowKey(it)
            const isRejecting = rejectingKey === key
            const detailHref = isVersion(kind)
              ? `/admin/${kind === 'mcp_version' ? 'mcp' : 'agents'}/${it.publisher_slug}/${it.entry_slug}`
              : `/admin/${kind === 'mcp_deletion' ? 'mcp' : 'agents'}/${it.publisher_slug}/${it.entry_slug}`
            return (
              <li key={key} className="p-4 space-y-2">
                <div className="flex items-start gap-3 flex-wrap">
                  <Badge variant="outline" className="text-xs flex items-center gap-1.5">
                    <Icon className="h-3 w-3" />
                    {kindLabel(kind)}
                  </Badge>
                  <a
                    href={detailHref}
                    className="text-sm font-mono text-primary hover:underline"
                  >
                    {it.publisher_slug}/{it.entry_slug}
                  </a>
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
                  <span className="text-xs text-muted-foreground ml-auto">
                    {formatDate(it.submitted_at)}
                  </span>
                </div>
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
                    onClick={() => approveMutation.mutate(it)}
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
    </div>
  )
}
