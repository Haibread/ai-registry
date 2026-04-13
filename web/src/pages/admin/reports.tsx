import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Flag, CheckCircle2, XCircle, Clock, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { EmptyState } from '@/components/ui/empty-state'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { formatDate } from '@/lib/utils'

type ReportStatus = 'pending' | 'reviewed' | 'dismissed'

const STATUS_TABS: { value: ReportStatus; label: string; icon: typeof Flag }[] = [
  { value: 'pending', label: 'Pending', icon: Clock },
  { value: 'reviewed', label: 'Reviewed', icon: CheckCircle2 },
  { value: 'dismissed', label: 'Dismissed', icon: XCircle },
]

function statusVariant(status: string): 'default' | 'secondary' | 'outline' {
  switch (status) {
    case 'pending':
      return 'default'
    case 'reviewed':
      return 'secondary'
    case 'dismissed':
      return 'outline'
    default:
      return 'outline'
  }
}

export default function AdminReports() {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<ReportStatus>('pending')
  const [actionError, setActionError] = useState<string | null>(null)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin-reports', statusFilter],
    queryFn: () =>
      api
        .GET('/api/v1/reports', { params: { query: { status: statusFilter } } })
        .then((r) => r.data),
    enabled: !!accessToken,
  })

  const items = data?.items ?? []

  const patchMutation = useMutation({
    mutationFn: async ({ id, status }: { id: string; status: ReportStatus }) => {
      setActionError(null)
      const { error } = await api.PATCH('/api/v1/reports/{id}', {
        params: { path: { id } },
        body: { status },
      })
      if (error) throw new Error((error as { detail?: string })?.detail || 'Failed to update')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-reports'] })
    },
    onError: (err: Error) => setActionError(err.message || 'Action failed'),
  })

  function linkFor(resourceType: string, resourceId: string) {
    // The admin detail pages take ns/slug, not IDs. Without a lookup we can
    // only point at the list pages filtered by ID-ish text. For now, link to
    // the admin list with a query so the reviewer can click through.
    if (resourceType === 'mcp_server') return `/admin/mcp?q=${encodeURIComponent(resourceId)}`
    if (resourceType === 'agent') return `/admin/agents?q=${encodeURIComponent(resourceId)}`
    return '#'
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Flag className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-2xl font-bold">Reports</h1>
      </div>
      <p className="text-sm text-muted-foreground">
        Community-submitted issue reports. Mark as reviewed once you've acted on them, or dismiss
        if they don't warrant action.
      </p>

      <div className="flex gap-2 border-b">
        {STATUS_TABS.map(({ value, label, icon: Icon }) => {
          const active = statusFilter === value
          return (
            <button
              key={value}
              type="button"
              onClick={() => setStatusFilter(value)}
              className={
                'flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors ' +
                (active
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground')
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          )
        })}
      </div>

      {actionError && (
        <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {actionError}
        </div>
      )}

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading reports…</p>
      ) : isError ? (
        <p className="text-sm text-destructive">Failed to load reports.</p>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Flag className="h-10 w-10" />}
          title={`No ${statusFilter} reports`}
          description={
            statusFilter === 'pending'
              ? 'You are all caught up — nothing to triage.'
              : `No reports in the ${statusFilter} state.`
          }
        />
      ) : (
        <ul className="divide-y rounded-md border">
          {items.map((r) => (
            <li key={r.id} className="p-4 space-y-2">
              <div className="flex items-start gap-3 flex-wrap">
                <Badge variant="outline" className="font-mono text-xs">
                  {r.issue_type}
                </Badge>
                <Badge variant="outline" className="text-xs">
                  {r.resource_type === 'mcp_server' ? 'MCP server' : 'Agent'}
                </Badge>
                <Badge variant={statusVariant(r.status)} className="text-xs">
                  {r.status}
                </Badge>
                <span className="text-xs text-muted-foreground">{formatDate(r.created_at)}</span>
                <a
                  href={linkFor(r.resource_type, r.resource_id)}
                  className="text-xs text-primary hover:underline font-mono ml-auto"
                >
                  {r.resource_id}
                </a>
              </div>
              <p className="whitespace-pre-wrap text-sm">{r.description}</p>
              {(r.reporter_ip || r.reviewed_by) && (
                <p className="text-xs text-muted-foreground">
                  {r.reporter_ip && <>Reporter: <span className="font-mono">{r.reporter_ip}</span></>}
                  {r.reporter_ip && r.reviewed_by && <> · </>}
                  {r.reviewed_by && <>Reviewed by: <span className="font-mono">{r.reviewed_by}</span></>}
                </p>
              )}
              <div className="flex gap-2 pt-1">
                {r.status !== 'reviewed' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => patchMutation.mutate({ id: r.id, status: 'reviewed' })}
                    disabled={patchMutation.isPending}
                  >
                    <CheckCircle2 className="h-4 w-4" />
                    <span className="ml-1.5">Mark reviewed</span>
                  </Button>
                )}
                {r.status !== 'dismissed' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => patchMutation.mutate({ id: r.id, status: 'dismissed' })}
                    disabled={patchMutation.isPending}
                  >
                    <XCircle className="h-4 w-4" />
                    <span className="ml-1.5">Dismiss</span>
                  </Button>
                )}
                {r.status !== 'pending' && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => patchMutation.mutate({ id: r.id, status: 'pending' })}
                    disabled={patchMutation.isPending}
                  >
                    <RotateCcw className="h-4 w-4" />
                    <span className="ml-1.5">Reopen</span>
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
