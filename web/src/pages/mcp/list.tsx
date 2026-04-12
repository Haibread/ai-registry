import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { ServerCard } from '@/components/mcp/server-card'
import { FilterBar } from '@/components/ui/filter-bar'
import { FilterBarSkeleton } from '@/components/ui/filter-bar-skeleton'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

export default function MCPListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined

  const api = getPublicClient()
  const { data, isLoading } = useQuery({
    queryKey: ['mcp-servers', { q, cursor, namespace, status }],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { q, cursor, limit: 20, namespace, status: status as 'draft' | 'published' | 'deprecated' | undefined } },
    }).then(r => r.data),
  })

  const servers = data?.items ?? []
  const hasFilters = !!(q || namespace || status)

  const buildNextParams = () => {
    const p = new URLSearchParams(searchParams)
    if (data?.next_cursor) p.set('cursor', data.next_cursor)
    else p.delete('cursor')
    return p.toString()
  }

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">Browse Model Context Protocol servers in the registry.</p>
        </div>

        {isLoading ? (
          <>
            <FilterBarSkeleton />
            <CardGridSkeleton count={6} />
          </>
        ) : (
          <>
            <FilterBar
              q={q}
              namespace={namespace}
              status={status}
              statusOptions={['published', 'deprecated']}
              searchPlaceholder="Search servers…"
            />

            {servers.length === 0 ? (
              <EmptyState
                icon={<ResourceIcon type="mcp-server" className="h-10 w-10" />}
                title={hasFilters ? 'No servers match your filters.' : 'No public MCP servers yet.'}
                description={hasFilters ? 'Try broadening your search or clearing filters.' : undefined}
                action={hasFilters ? (
                  <Button variant="outline" size="sm" onClick={() => setSearchParams({})}>Clear filters</Button>
                ) : undefined}
              />
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  Showing {servers.length}{data?.total_count && data.total_count > servers.length ? ` of ${data.total_count}` : ''} server{servers.length !== 1 ? 's' : ''}
                </p>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {servers.map((s) => <ServerCard key={s.id} server={s} />)}
                </div>
              </>
            )}

            {data?.next_cursor && (
              <div className="flex justify-center">
                <Button variant="outline" asChild>
                  <Link to={`/mcp?${buildNextParams()}`}>Load more</Link>
                </Button>
              </div>
            )}
          </>
        )}
      </main>
      <Footer />
    </div>
  )
}
