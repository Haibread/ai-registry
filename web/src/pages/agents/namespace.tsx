/**
 * AgentNamespacePage — per-publisher landing page for agents.
 *
 * Route: /agents/:namespace
 *
 * Mirrors `pages/mcp/namespace.tsx` for the agent half of the registry.
 * Uses the existing `namespace=X` filter on `GET /api/v1/agents` — no new
 * endpoint. See that file's header comment for the loading/empty/404
 * state machine; it is identical here.
 */

import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { CheckCircle } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { AgentCard } from '@/components/agents/agent-card'
import { getPublicClient } from '@/lib/api-client'

export default function AgentNamespacePage() {
  const { namespace } = useParams<{ namespace: string }>()
  const api = getPublicClient()

  // See `pages/mcp/namespace.tsx` for the `?? null` rationale — react-query
  // rejects `undefined` results, so we translate openapi-fetch's non-2xx
  // `{ data: undefined }` into a null sentinel.
  const { data: publisher, isLoading: publisherLoading } = useQuery({
    queryKey: ['publisher', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{slug}', {
          params: { path: { slug: namespace! } },
        })
        .then((r) => r.data ?? null),
    enabled: !!namespace,
  })

  const { data: listData, isLoading: listLoading } = useQuery({
    queryKey: ['agents-ns', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/agents', {
          params: { query: { namespace: namespace!, limit: 50 } },
        })
        .then((r) => r.data ?? null),
    enabled: !!namespace,
  })

  if (publisherLoading || listLoading) {
    return (
      <div className="flex min-h-screen flex-col">
        <Header />
        <main className="flex-1 container py-8 space-y-6">
          <CardGridSkeleton count={6} />
        </main>
        <Footer />
      </div>
    )
  }

  if (!publisher) {
    return (
      <div className="flex min-h-screen flex-col">
        <Header />
        <main className="flex-1 container py-8 max-w-4xl">
          <EmptyState
            icon={<ResourceIcon type="publisher" className="h-10 w-10" />}
            title="Namespace not found"
            description={`No publisher is registered under "${namespace}".`}
            action={
              <Button variant="outline" size="sm" asChild>
                <Link to="/agents">Browse all agents</Link>
              </Button>
            }
          />
        </main>
        <Footer />
      </div>
    )
  }

  const agents = listData?.items ?? []
  const totalCount = listData?.total_count ?? agents.length

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'Agents', href: '/agents' },
            { label: publisher.slug },
          ]}
        />

        <div className="space-y-2">
          <div className="flex items-center gap-3 flex-wrap">
            <ResourceIcon type="publisher" className="h-6 w-6 text-muted-foreground" />
            <h1 className="text-2xl font-bold">{publisher.name}</h1>
            {publisher.verified && (
              <Badge variant="default" className="flex items-center gap-1">
                <CheckCircle className="h-3 w-3" /> Verified
              </Badge>
            )}
            <Button variant="ghost" size="sm" asChild className="ml-auto">
              <Link to={`/publishers/${publisher.slug}`}>View publisher profile →</Link>
            </Button>
          </div>
          <p className="text-sm text-muted-foreground">
            Agents published by <span className="font-mono">{publisher.slug}</span>.
          </p>
        </div>

        {agents.length === 0 ? (
          <EmptyState
            icon={<ResourceIcon type="agent" className="h-10 w-10" />}
            title="No agents yet"
            description={`${publisher.name} hasn't published any agents yet. Check back soon, or browse other publishers.`}
            action={
              <Button variant="outline" size="sm" asChild>
                <Link to="/agents">Browse all agents</Link>
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-sm text-muted-foreground">
              Showing {agents.length}
              {totalCount > agents.length ? ` of ${totalCount}` : ''} agent
              {agents.length !== 1 ? 's' : ''}
            </p>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {agents.map((a) => (
                <AgentCard key={a.id} agent={a} />
              ))}
            </div>
          </>
        )}
      </main>
      <Footer />
    </div>
  )
}
