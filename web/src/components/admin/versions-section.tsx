import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { GitPullRequestArrow, CheckCircle2, AlertCircle, Send, Undo2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { useAuthClient } from '@/lib/api-client'
import { usePermissions } from '@/auth/useMe'
import { formatDate } from '@/lib/utils'
import type { components } from '@/lib/schema'

type MCPVersion = components['schemas']['MCPServerVersion']
type AgentVersion = components['schemas']['AgentVersion']
type AnyVersion = MCPVersion | AgentVersion

type Kind = 'mcp' | 'agent'

interface VersionsSectionProps {
  kind: Kind
  namespace: string
  slug: string
}

// review_state → badge variant + label.
function reviewStateBadge(state: string | undefined) {
  switch (state) {
    case 'pending_review':
      return { variant: 'default' as const, label: 'pending review' }
    case 'rejected':
      return { variant: 'destructive' as const, label: 'rejected' }
    case 'none':
    default:
      return null
  }
}

function publishStatus(v: AnyVersion) {
  if (v.published_at) return { variant: 'secondary' as const, label: 'published' }
  return { variant: 'outline' as const, label: 'draft' }
}

// Map RFC 7807 type-URI suffix → friendly UX message.
function friendlyProblem(error: unknown, fallback: string): string {
  const e = error as { type?: string; detail?: string }
  if (!e?.type) return e?.detail ?? fallback
  if (e.type.endsWith('review-state-mismatch'))
    return 'The version is no longer in the expected state — refresh and try again.'
  if (e.type.endsWith('review-revision-mismatch'))
    return 'The version was edited since this page loaded — refresh.'
  if (e.type.endsWith('already-published'))
    return 'The version is already published.'
  if (e.type.endsWith('review-already-pending'))
    return 'Another version on this entry is already pending review.'
  return e.detail ?? fallback
}

export function VersionsSection({ kind, namespace, slug }: VersionsSectionProps) {
  const perms = usePermissions()
  // Submit/withdraw are publisher Editor actions. Approve/reject are
  // Reviewer actions and live on the review queue, not here.
  const canEdit = perms.canEdit(namespace)
  const api = useAuthClient()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState<string | null>(null)

  const queryKey = ['admin-versions', kind, namespace, slug]

  const { data, isPending, isError } = useQuery<{ items?: AnyVersion[] }>({
    queryKey,
    queryFn: async () => {
      if (kind === 'mcp') {
        const r = await api.GET('/api/v1/mcp/servers/{namespace}/{slug}/versions', {
          params: { path: { namespace, slug } },
        })
        return (r.data ?? { items: [] }) as { items?: AnyVersion[] }
      }
      const r = await api.GET('/api/v1/agents/{namespace}/{slug}/versions', {
        params: { path: { namespace, slug } },
      })
      return (r.data ?? { items: [] }) as { items?: AnyVersion[] }
    },
    enabled: true,
  })

  const items: AnyVersion[] = data?.items ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey })

  // ── Submit / withdraw mutations (publisher actions) ────────────────────

  const submitMutation = useMutation({
    mutationFn: async (version: string) => {
      setActionError(null)
      // openapi-fetch's POST is string-literal-typed per path; the two
      // branches are structurally identical (no body, same params).
      const result =
        kind === 'mcp'
          ? await api.POST(
              '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
              { params: { path: { namespace, slug, version } } },
            )
          : await api.POST(
              '/api/v1/agents/{namespace}/{slug}/versions/{version}/submit',
              { params: { path: { namespace, slug, version } } },
            )
      if (result.error) throw new Error(friendlyProblem(result.error, 'Submit failed.'))
      return version
    },
    onSuccess: (version) => {
      toast.success(`v${version} sent for review`)
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const withdrawMutation = useMutation({
    mutationFn: async (version: string) => {
      setActionError(null)
      const result =
        kind === 'mcp'
          ? await api.POST(
              '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/withdraw',
              { params: { path: { namespace, slug, version } } },
            )
          : await api.POST(
              '/api/v1/agents/{namespace}/{slug}/versions/{version}/withdraw',
              { params: { path: { namespace, slug, version } } },
            )
      if (result.error) throw new Error(friendlyProblem(result.error, 'Withdraw failed.'))
      return version
    },
    onSuccess: (version) => {
      toast.success(`v${version} withdrawn`)
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setActionError(err.message),
  })

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <GitPullRequestArrow className="h-4 w-4" aria-hidden="true" />
          Versions
          <span className="text-sm font-normal text-muted-foreground">({items.length})</span>
        </h2>
      </div>
      <p className="text-sm text-muted-foreground">
        Drafts are authored here and sent for reviewer approval.
        Approve and reject happen on the{' '}
        <a href="/admin/review" className="text-primary hover:underline">
          review queue
        </a>
        .
      </p>

      {actionError && (
        <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {actionError}
        </div>
      )}

      {isPending ? (
        <TableSkeleton rows={3} cols={5} />
      ) : isError ? (
        <p className="text-sm text-destructive py-4">Failed to load versions.</p>
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground py-4">No versions yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Version</TableHead>
              <TableHead>State</TableHead>
              <TableHead>Submitted</TableHead>
              <TableHead>Reviewed</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((v) => {
              const reviewBadge = reviewStateBadge(v.review_state)
              const publishBadge = publishStatus(v)
              const isPending = v.review_state === 'pending_review'
              const isDraftish =
                v.review_state === 'rejected' ||
                (!v.published_at && (!v.review_state || v.review_state === 'none'))
              return (
                <TableRow key={v.version}>
                  <TableCell className="font-mono">
                    v{v.version}
                    {typeof v.revision === 'number' && v.revision > 0 && (
                      <Badge variant="outline" className="ml-2 text-xs">
                        rev {v.revision}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="space-x-1.5">
                    <Badge variant={publishBadge.variant} className="text-xs">
                      {publishBadge.label}
                    </Badge>
                    {reviewBadge && (
                      <Badge variant={reviewBadge.variant} className="text-xs">
                        {reviewBadge.label}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-sm">
                    {v.submitted_at ? (
                      <>
                        <div>{formatDate(v.submitted_at)}</div>
                        {v.submitted_by_email && (
                          <div className="text-xs text-muted-foreground font-mono">
                            {v.submitted_by_email}
                          </div>
                        )}
                      </>
                    ) : (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-sm">
                    {v.reviewed_at ? (
                      <>
                        <div className="flex items-center gap-1">
                          {v.review_decision === 'approved' ? (
                            <CheckCircle2 className="h-3.5 w-3.5 text-green-600" />
                          ) : v.review_decision === 'rejected' ? (
                            <AlertCircle className="h-3.5 w-3.5 text-destructive" />
                          ) : null}
                          {formatDate(v.reviewed_at)}
                        </div>
                        {v.reviewed_by_email && (
                          <div className="text-xs text-muted-foreground font-mono">
                            {v.reviewed_by_email}
                          </div>
                        )}
                        {v.review_decision === 'rejected' && v.rejection_reason && (
                          <div className="mt-1 text-xs text-destructive whitespace-pre-wrap">
                            <span className="font-semibold">Reason:</span> {v.rejection_reason}
                          </div>
                        )}
                      </>
                    ) : (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      {canEdit && isDraftish && !v.published_at && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => submitMutation.mutate(v.version)}
                          disabled={submitMutation.isPending || withdrawMutation.isPending}
                        >
                          <Send className="h-4 w-4" />
                          <span className="ml-1.5">
                            {v.review_state === 'rejected' ? 'Resubmit' : 'Submit'}
                          </span>
                        </Button>
                      )}
                      {canEdit && isPending && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => withdrawMutation.mutate(v.version)}
                          disabled={submitMutation.isPending || withdrawMutation.isPending}
                        >
                          <Undo2 className="h-4 w-4" />
                          <span className="ml-1.5">Withdraw</span>
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
