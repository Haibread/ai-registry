import { useState } from 'react'
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Plus, ArrowRight, Server } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { FilterBar } from '@/components/ui/filter-bar'
import { BulkActionBar } from '@/components/admin/bulk-action-bar'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Checkbox } from '@/components/ui/checkbox'
import { useBulkSelection } from '@/hooks/use-bulk-selection'
import { useAuthClient } from '@/lib/api-client'
import { cn, formatDate, problemMessage } from '@/lib/utils'
import { usePermissions } from '@/auth/useMe'
import { usePublisher } from '@/auth/PublisherContext'

const PAGE_LIMIT = 50

// Mirrors the API's `sort` enum; '' falls back to the server default
// (created_at_desc). Search results stay relevance-ranked regardless.
const SORT_OPTIONS = [
  { value: '', label: 'Newest first' },
  { value: 'updated_at_desc', label: 'Recently updated' },
  { value: 'published_at_desc', label: 'Recently published' },
  { value: 'name_asc', label: 'Name A–Z' },
  { value: 'name_desc', label: 'Name Z–A' },
]

export default function AdminMCPList() {
  const perms = usePermissions()
  const api = useAuthClient()
  const { currentSlug } = usePublisher()
  const [searchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined
  const visibility = searchParams.get('visibility') ?? undefined
  const sort = searchParams.get('sort') ?? undefined
  const hasFilters = !!(q || namespace || status || visibility)
  // Scope the list to the publisher the admin area is currently focused on.
  // An explicit FilterBar publisher filter wins; a Server Admin's
  // All-publishers view (currentSlug = null) leaves the list unscoped.
  const effectiveNamespace = namespace ?? currentSlug ?? undefined
  // Infinite pagination: each "Load more" appends the next page rather than
  // replacing the visible rows (the old cursor-in-URL approach hid earlier rows).
  const { data, isPending, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey: ['admin-mcp', { q, namespace: effectiveNamespace, status, visibility, sort }],
      queryFn: async ({ pageParam }) => {
        const { data, error } = await api.GET('/api/v1/mcp/servers', {
          params: {
            query: {
              limit: PAGE_LIMIT,
              // mine=true scopes the list to the publishers the caller manages:
              // Server Admins still see everything, authors see only
              // their own resources (incl. private drafts).
              mine: true,
              q,
              namespace: effectiveNamespace,
              cursor: pageParam || undefined,
              status: status as 'draft' | 'published' | 'deprecated' | undefined,
              visibility: visibility as 'public' | 'private' | undefined,
              sort: sort as 'created_at_desc' | 'updated_at_desc' | 'published_at_desc' | 'name_asc' | 'name_desc' | undefined,
            },
          },
        })
        // openapi-fetch resolves (not rejects) on HTTP errors, so surface them
        // to React Query — otherwise the list silently renders empty instead of
        // the error state.
        if (error) throw new Error(problemMessage(error, 'Failed to load MCP servers.'))
        return data
      },
      initialPageParam: '',
      getNextPageParam: (last) => last?.next_cursor || undefined,
      enabled: true,
    })

  const servers = data?.pages.flatMap((p) => p?.items ?? []) ?? []
  const totalCount = data?.pages[0]?.total_count
  const queryClient = useQueryClient()
  const selection = useBulkSelection<{ id: string; namespace: string; slug: string }>()
  const [bulkError, setBulkError] = useState<string | null>(null)

  const selectedItems = servers.filter((s) => selection.isSelected(s.id))

  const bulkMutation = useMutation({
    // Run the per-item calls concurrently and tolerate partial failure: one
    // bad item should neither block the rest nor hide which ones succeeded.
    mutationFn: async (action: (s: { namespace: string; slug: string }) => Promise<void>) => {
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
      queryClient.invalidateQueries({ queryKey: ['admin-mcp'] })
      selection.clear()
    },
    // Refresh even on partial failure so succeeded items reflect their new
    // state; keep the selection so the user can retry the ones that failed.
    onError: (err: Error) => {
      queryClient.invalidateQueries({ queryKey: ['admin-mcp'] })
      setBulkError(err.message || 'Bulk action failed')
    },
  })

  const bulkSetVisibility = (visibility: 'public' | 'private') => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.POST('/api/v1/mcp/servers/{namespace}/{slug}/visibility', {
        params: { path: { namespace, slug } },
        body: { visibility },
      })
      if (error) throw new Error(problemMessage(error, 'Visibility change failed.'))
    })
  }

  // Bulk deprecate/delete confirm through the shared dialog (P2.1) — the
  // pending kind also selects the dialog copy.
  const [bulkConfirm, setBulkConfirm] = useState<'deprecate' | 'delete' | null>(null)

  const bulkDeprecate = () => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.POST('/api/v1/mcp/servers/{namespace}/{slug}/deprecate', {
        params: { path: { namespace, slug } },
      })
      if (error) throw new Error(problemMessage(error, 'Deprecate failed.'))
    })
  }

  const bulkDelete = () => {
    bulkMutation.mutate(async ({ namespace, slug }) => {
      const { error } = await api.DELETE('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace, slug } },
      })
      if (error) throw new Error(problemMessage(error, 'Delete failed.'))
    })
  }

  return (
    // pb-24 keeps the floating bulk bar from covering the last table row
    // while a selection is active.
    <div className={cn('space-y-4 max-w-6xl mx-auto', selection.selectedCount > 0 && 'pb-24')}>
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">
            {servers.length}{totalCount && totalCount > servers.length ? ` of ${totalCount}` : ''} entr{servers.length !== 1 ? 'ies' : 'y'}
          </p>
        </div>
        {perms.isEditorAnywhere && (
          <Button asChild>
            <Link to="/admin/mcp/new" className="flex items-center gap-1.5">
              <Plus className="h-4 w-4" /> New Server
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
        searchPlaceholder="Search servers…"
        sortOptions={SORT_OPTIONS}
        // When the admin area is scoped to one publisher the free-text
        // publisher filter is redundant and could contradict the scope.
        showPublisherFilter={!currentSlug}
      />

      {isPending ? (
        <TableSkeleton rows={6} cols={7} />
      ) : isError ? (
        <ErrorState message="Failed to load MCP servers." />
      ) : servers.length === 0 ? (
        hasFilters ? (
          <EmptyState
            icon={<Server className="h-10 w-10" aria-hidden="true" />}
            title="No servers match your filters."
            description="Try clearing or adjusting the filters above."
          />
        ) : (
          <EmptyState
            icon={<Server className="h-10 w-10" aria-hidden="true" />}
            title="No MCP servers yet."
            description="Register your first MCP server to get started."
            action={
              perms.isEditorAnywhere ? (
                <Button asChild size="sm">
                  <Link to="/admin/mcp/new">New Server</Link>
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
                  <Checkbox
                    aria-label="Select all"
                    checked={servers.length > 0 && servers.every((s) => selection.isSelected(s.id))}
                    indeterminate={selection.selectedCount > 0 && !servers.every((s) => selection.isSelected(s.id))}
                    onChange={() => selection.toggleAll(servers)}
                  />
                </TableHead>
                <TableHead>Name</TableHead>
                <TableHead className="hidden @2xl:table-cell">Namespace / Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="hidden @3xl:table-cell">Visibility</TableHead>
                <TableHead className="hidden @4xl:table-cell">Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {servers.map((s) => (
                <TableRow key={s.id} data-state={selection.isSelected(s.id) ? 'selected' : undefined}>
                  <TableCell>
                    <Checkbox
                      aria-label={`Select ${s.name}`}
                      checked={selection.isSelected(s.id)}
                      onChange={() => selection.toggle(s.id)}
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    {/* The name is the row's primary navigation — Manage stays
                        as a secondary affordance but may be off-screen on
                        narrow containers. */}
                    <Link
                      to={`/admin/mcp/${s.namespace}/${s.slug}`}
                      className="hover:underline underline-offset-4 focus-visible:underline"
                    >
                      {s.name}
                    </Link>
                    {/* Show the namespace/slug inline on narrow containers
                        where its dedicated column is hidden. */}
                    <div className="font-mono text-xs text-muted-foreground @2xl:hidden">
                      {s.namespace}/{s.slug}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground hidden @2xl:table-cell">
                    {s.namespace}/{s.slug}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={s.status} />
                  </TableCell>
                  <TableCell className="hidden @3xl:table-cell">
                    <VisibilityBadge visibility={s.visibility} />
                  </TableCell>
                  <TableCell className="text-muted-foreground whitespace-nowrap hidden @4xl:table-cell">{formatDate(s.updated_at)}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" size="sm" asChild>
                      <Link to={`/admin/mcp/${s.namespace}/${s.slug}`}>
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
            onDeprecate={() => setBulkConfirm('deprecate')}
            onDelete={() => setBulkConfirm('delete')}
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

      <ConfirmDialog
        open={bulkConfirm !== null}
        onOpenChange={(o) => { if (!o) setBulkConfirm(null) }}
        title={
          bulkConfirm === 'delete'
            ? `Delete ${selection.selectedCount} server${selection.selectedCount === 1 ? '' : 's'}?`
            : `Deprecate ${selection.selectedCount} server${selection.selectedCount === 1 ? '' : 's'}?`
        }
        description={
          bulkConfirm === 'delete'
            ? 'This permanently deletes the selected entries and all their versions. It cannot be undone.'
            : 'This marks the selected entries as deprecated, signalling consumers to migrate away. They can be republished later.'
        }
        confirmLabel={bulkConfirm === 'delete' ? 'Delete' : 'Deprecate'}
        destructive={bulkConfirm === 'delete'}
        isPending={bulkMutation.isPending}
        onConfirm={() => {
          if (bulkConfirm === 'delete') bulkDelete()
          else bulkDeprecate()
          setBulkConfirm(null)
        }}
      />
    </div>
  )
}
