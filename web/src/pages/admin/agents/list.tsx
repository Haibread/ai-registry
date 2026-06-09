import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Plus, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { FilterBar } from '@/components/ui/filter-bar'
import { BulkActionBar } from '@/components/admin/bulk-action-bar'
import { useBulkSelection } from '@/hooks/use-bulk-selection'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { usePermissions } from '@/auth/useMe'
import { usePublisher } from '@/auth/PublisherContext'

const PAGE_LIMIT = 50

export default function AdminAgentList() {
  const perms = usePermissions()
  const api = useAuthClient()
  const { currentSlug } = usePublisher()
  const [searchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined
  const visibility = searchParams.get('visibility') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined
  // Scope the list to the publisher the admin area is currently focused on.
  // An explicit FilterBar publisher filter wins; a Server Admin's
  // All-publishers view (currentSlug = null) leaves the list unscoped.
  const effectiveNamespace = namespace ?? currentSlug ?? undefined
  const { data } = useQuery({
    queryKey: ['admin-agents', { q, namespace: effectiveNamespace, status, visibility, cursor }],
    queryFn: () => api.GET('/api/v1/agents', {
      params: {
        query: {
          limit: PAGE_LIMIT,
          // mine=true scopes the list to the publishers the caller manages;
          // Server Admins still see everything.
          mine: true,
          q,
          namespace: effectiveNamespace,
          cursor,
          status: status as 'draft' | 'published' | 'deprecated' | undefined,
          visibility: visibility as 'public' | 'private' | undefined,
        },
      },
    }).then(r => r.data),
    enabled: true,
  })

  // Defensive client-side filter: older server builds return soft-deleted
  // agents (status='deleted') from /api/v1/agents when no status filter is
  // passed. Newer server builds exclude them by default at the store layer,
  // but the admin UI should never surface tombstoned rows regardless.
  // Cast through string: 'deleted' is no longer in the published schema union,
  // but older server builds may still return it. The defensive filter must
  // outlive the type narrowing, so we widen the comparison explicitly.
  const agents = (data?.items ?? []).filter((a) => (a.status as string) !== 'deleted')
  const queryClient = useQueryClient()
  const selection = useBulkSelection<{ id: string; namespace: string; slug: string }>()
  const [bulkError, setBulkError] = useState<string | null>(null)

  const selectedItems = agents.filter((a) => selection.isSelected(a.id))

  const bulkMutation = useMutation({
    mutationFn: async (action: (a: { namespace: string; slug: string }) => Promise<void>) => {
      setBulkError(null)
      for (const item of selectedItems) {
        await action({ namespace: item.namespace, slug: item.slug })
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
      selection.clear()
    },
    onError: (err: Error) => setBulkError(err.message || 'Bulk action failed'),
  })

  const bulkSetVisibility = (visibility: 'public' | 'private') => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      await api.POST('/api/v1/agents/{namespace}/{slug}/visibility', {
        params: { path: { namespace, slug } },
        body: { visibility },
      })
    })
  }

  const bulkDeprecate = () => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      await api.POST('/api/v1/agents/{namespace}/{slug}/deprecate', {
        params: { path: { namespace, slug } },
      })
    })
  }

  const bulkDelete = () => {
    if (!window.confirm(`Delete ${selection.selectedCount} agent(s)? This cannot be undone.`)) return
    bulkMutation.mutate(async ({ namespace, slug }) => {
      await api.DELETE('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace, slug } },
      })
    })
  }

  const nextParams = new URLSearchParams()
  if (q) nextParams.set('q', q)
  if (namespace) nextParams.set('namespace', namespace)
  if (status) nextParams.set('status', status)
  if (visibility) nextParams.set('visibility', visibility)
  if (data?.next_cursor) nextParams.set('cursor', data.next_cursor)

  return (
    <div className="space-y-4 max-w-5xl mx-auto">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">Agents</h1>
          <p className="text-muted-foreground mt-1">
            {agents.length}{data?.total_count && data.total_count > agents.length ? ` of ${data.total_count}` : ''} entr{agents.length !== 1 ? 'ies' : 'y'}
          </p>
        </div>
        {perms.isEditorAnywhere && (
          <Button asChild>
            <Link to="/admin/agents/new" className="flex items-center gap-1.5">
              <Plus className="h-4 w-4" /> New Agent
            </Link>
          </Button>
        )}
      </div>

      <FilterBar
        q={q}
        namespace={namespace}
        status={status}
        visibility={visibility}
        statusOptions={['draft', 'published', 'deprecated']}
        showVisibility
        searchPlaceholder="Search agents…"
      />

      {agents.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">
          {q || namespace || status || visibility
            ? 'No agents match your filters.'
            : 'No agents yet.'}
        </p>
      ) : (
        <>
          {bulkError && (
            <div role="alert" className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {bulkError}
            </div>
          )}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-8">
                  <input
                    type="checkbox"
                    aria-label="Select all"
                    checked={agents.length > 0 && agents.every((a) => selection.isSelected(a.id))}
                    onChange={() => selection.toggleAll(agents)}
                  />
                </TableHead>
                <TableHead>Name</TableHead>
                <TableHead className="hidden sm:table-cell">Namespace / Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="hidden md:table-cell">Visibility</TableHead>
                <TableHead className="hidden lg:table-cell">Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((a) => (
                <TableRow key={a.id} data-selected={selection.isSelected(a.id) || undefined}>
                  <TableCell>
                    <input
                      type="checkbox"
                      aria-label={`Select ${a.name}`}
                      checked={selection.isSelected(a.id)}
                      onChange={() => selection.toggle(a.id)}
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    {a.name}
                    <div className="font-mono text-xs text-muted-foreground sm:hidden">
                      {a.namespace}/{a.slug}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground hidden sm:table-cell">
                    {a.namespace}/{a.slug}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={a.status} />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <VisibilityBadge visibility={a.visibility} />
                  </TableCell>
                  <TableCell className="text-muted-foreground hidden lg:table-cell">{formatDate(a.updated_at)}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" size="sm" asChild>
                      <Link to={`/admin/agents/${a.namespace}/${a.slug}`}>
                        Manage
                        <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                      </Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          <BulkActionBar
            selectedCount={selection.selectedCount}
            onClear={selection.clear}
            onSetVisibility={bulkSetVisibility}
            onDeprecate={bulkDeprecate}
            onDelete={bulkDelete}
            isBusy={bulkMutation.isPending}
            // Visibility + deprecate are Editor/Admin actions; the direct
            // delete escape hatch stays Server-Admin-only. The backend still
            // enforces per-resource (incl. "public requires an approved version").
            canSetVisibility={perms.isEditorAnywhere}
            canDelete={perms.isServerAdmin}
            canDeprecate={perms.isEditorAnywhere}
          />

          {data?.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" asChild>
                <Link to={`/admin/agents?${nextParams.toString()}`}>Load more</Link>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
