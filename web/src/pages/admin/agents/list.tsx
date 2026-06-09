import { useState } from 'react'
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Plus, ArrowRight, Bot } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { FilterBar } from '@/components/ui/filter-bar'
import { BulkActionBar } from '@/components/admin/bulk-action-bar'
import { useBulkSelection } from '@/hooks/use-bulk-selection'
import { useAuthClient } from '@/lib/api-client'
import { formatDate, problemMessage } from '@/lib/utils'
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
  const hasFilters = !!(q || namespace || status || visibility)
  // Scope the list to the publisher the admin area is currently focused on.
  // An explicit FilterBar publisher filter wins; a Server Admin's
  // All-publishers view (currentSlug = null) leaves the list unscoped.
  const effectiveNamespace = namespace ?? currentSlug ?? undefined
  // Infinite pagination: each "Load more" appends the next page rather than
  // replacing the visible rows (the old cursor-in-URL approach hid earlier rows).
  const { data, isPending, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey: ['admin-agents', { q, namespace: effectiveNamespace, status, visibility }],
      queryFn: async ({ pageParam }) => {
        const { data, error } = await api.GET('/api/v1/agents', {
          params: {
            query: {
              limit: PAGE_LIMIT,
              // mine=true scopes the list to the publishers the caller manages;
              // Server Admins still see everything.
              mine: true,
              q,
              namespace: effectiveNamespace,
              cursor: pageParam || undefined,
              status: status as 'draft' | 'published' | 'deprecated' | undefined,
              visibility: visibility as 'public' | 'private' | undefined,
            },
          },
        })
        // openapi-fetch resolves (not rejects) on HTTP errors, so surface them
        // to React Query — otherwise the list silently renders empty instead of
        // the error state.
        if (error) throw new Error(problemMessage(error, 'Failed to load agents.'))
        return data
      },
      initialPageParam: '',
      getNextPageParam: (last) => last?.next_cursor || undefined,
      enabled: true,
    })

  // Defensive client-side filter: older server builds return soft-deleted
  // agents (status='deleted') from /api/v1/agents when no status filter is
  // passed. Newer server builds exclude them by default at the store layer,
  // but the admin UI should never surface tombstoned rows regardless.
  // Cast through string: 'deleted' is no longer in the published schema union,
  // but older server builds may still return it. The defensive filter must
  // outlive the type narrowing, so we widen the comparison explicitly.
  const agents = (data?.pages.flatMap((p) => p?.items ?? []) ?? []).filter(
    (a) => (a.status as string) !== 'deleted',
  )
  const totalCount = data?.pages[0]?.total_count
  const queryClient = useQueryClient()
  const selection = useBulkSelection<{ id: string; namespace: string; slug: string }>()
  const [bulkError, setBulkError] = useState<string | null>(null)

  const selectedItems = agents.filter((a) => selection.isSelected(a.id))

  const bulkMutation = useMutation({
    // Run the per-item calls concurrently and tolerate partial failure: one
    // bad item should neither block the rest nor hide which ones succeeded.
    mutationFn: async (action: (a: { namespace: string; slug: string }) => Promise<void>) => {
      setBulkError(null)
      const results = await Promise.allSettled(
        selectedItems.map((item) => action({ namespace: item.namespace, slug: item.slug })),
      )
      const failed = results.filter((r) => r.status === 'rejected').length
      if (failed > 0) {
        throw new Error(`${selectedItems.length - failed} of ${selectedItems.length} succeeded; ${failed} failed.`)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
      selection.clear()
    },
    // Refresh even on partial failure so succeeded items reflect their new
    // state; keep the selection so the user can retry the ones that failed.
    onError: (err: Error) => {
      queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
      setBulkError(err.message || 'Bulk action failed')
    },
  })

  const bulkSetVisibility = (visibility: 'public' | 'private') => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/visibility', {
        params: { path: { namespace, slug } },
        body: { visibility },
      })
      if (error) throw new Error(problemMessage(error, 'Visibility change failed.'))
    })
  }

  const bulkDeprecate = () => {
    if (!window.confirm(`Deprecate ${selection.selectedCount} agent(s)? This signals consumers to migrate away.`)) return
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.POST('/api/v1/agents/{namespace}/{slug}/deprecate', {
        params: { path: { namespace, slug } },
      })
      if (error) throw new Error(problemMessage(error, 'Deprecate failed.'))
    })
  }

  const bulkDelete = () => {
    if (!window.confirm(`Delete ${selection.selectedCount} agent(s)? This cannot be undone.`)) return
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.DELETE('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace, slug } },
      })
      if (error) throw new Error(problemMessage(error, 'Delete failed.'))
    })
  }

  return (
    <div className="space-y-4 max-w-5xl mx-auto">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">Agents</h1>
          <p className="text-muted-foreground mt-1">
            {agents.length}{totalCount && totalCount > agents.length ? ` of ${totalCount}` : ''} entr{agents.length !== 1 ? 'ies' : 'y'}
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

      {isPending ? (
        <TableSkeleton rows={6} cols={7} />
      ) : isError ? (
        <ErrorState message="Failed to load agents." />
      ) : agents.length === 0 ? (
        hasFilters ? (
          <EmptyState
            icon={<Bot className="h-10 w-10" aria-hidden="true" />}
            title="No agents match your filters."
            description="Try clearing or adjusting the filters above."
          />
        ) : (
          <EmptyState
            icon={<Bot className="h-10 w-10" aria-hidden="true" />}
            title="No agents yet."
            description="Register your first agent to get started."
            action={
              perms.isEditorAnywhere ? (
                <Button asChild size="sm">
                  <Link to="/admin/agents/new">New Agent</Link>
                </Button>
              ) : undefined
            }
          />
        )
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

          {hasNextPage && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" onClick={() => fetchNextPage()} disabled={isFetchingNextPage}>
                {isFetchingNextPage ? 'Loading…' : 'Load more'}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
