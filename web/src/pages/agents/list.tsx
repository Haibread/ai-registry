import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Bot } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { AgentCard } from '@/components/agents/agent-card'
import { FilterBar } from '@/components/ui/filter-bar'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

export default function AgentListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined

  const api = getPublicClient()
  const { data } = useQuery({
    queryKey: ['agents', { q, cursor, namespace, status }],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { q, cursor, limit: 20, namespace, status: status as 'draft' | 'published' | 'deprecated' | undefined } },
    }).then(r => r.data),
  })

  const agents = data?.items ?? []

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

        <FilterBar
          q={q}
          namespace={namespace}
          status={status}
          statusOptions={['published', 'deprecated']}
          searchPlaceholder="Search agents…"
        />

        {agents.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-16 text-center">
            <Bot className="h-10 w-10 text-muted-foreground/40" aria-hidden="true" />
            <p className="text-muted-foreground font-medium">
              {q || namespace || status ? 'No agents match your filters.' : 'No public agents yet.'}
            </p>
            {(q || namespace || status) && (
              <Button variant="outline" size="sm" onClick={() => setSearchParams({})}>Clear filters</Button>
            )}
          </div>
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
      </main>
      <Footer />
    </div>
  )
}
