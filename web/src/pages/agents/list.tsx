import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { AgentCard } from '@/components/agents/agent-card'
import { FilterBar } from '@/components/ui/filter-bar'
import { FilterBarSkeleton } from '@/components/ui/filter-bar-skeleton'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

export default function AgentListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined
  const sort = searchParams.get('sort') ?? undefined

  const api = getPublicClient()
  const { data, isLoading } = useQuery({
    queryKey: ['agents', { q, cursor, namespace, status, sort }],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { q, cursor, limit: 20, namespace, status: status as 'draft' | 'published' | 'deprecated' | undefined, sort: sort as 'created_at_desc' | 'updated_at_desc' | 'name_asc' | 'name_desc' | undefined } },
    }).then(r => r.data),
  })

  const agents = data?.items ?? []
  const hasFilters = !!(q || namespace || status || sort)

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
          <h1 className="text-2xl font-bold">AI Agents</h1>
          <p className="text-muted-foreground mt-1">Browse A2A-compatible agents in the registry.</p>
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
              searchPlaceholder="Search agents…"
              sortOptions={[
                { value: '', label: 'Newest first' },
                { value: 'updated_at_desc', label: 'Recently updated' },
                { value: 'name_asc', label: 'Name A–Z' },
                { value: 'name_desc', label: 'Name Z–A' },
              ]}
            />

            {agents.length === 0 ? (
              <EmptyState
                icon={<ResourceIcon type="agent" className="h-10 w-10" />}
                title={hasFilters ? 'No agents match your filters.' : 'No public agents yet.'}
                description={hasFilters ? 'Try broadening your search or clearing filters.' : undefined}
                action={hasFilters ? (
                  <Button variant="outline" size="sm" onClick={() => setSearchParams({})}>Clear filters</Button>
                ) : undefined}
              />
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  Showing {agents.length}{data?.total_count && data.total_count > agents.length ? ` of ${data.total_count}` : ''} agent{agents.length !== 1 ? 's' : ''}
                </p>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {agents.map((a) => <AgentCard key={a.id} agent={a} />)}
                </div>
              </>
            )}

            {data?.next_cursor && (
              <div className="flex justify-center">
                <Button variant="outline" asChild>
                  <Link to={`/agents?${buildNextParams()}`}>Load more</Link>
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
