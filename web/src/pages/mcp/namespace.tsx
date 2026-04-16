/**
 * MCPNamespacePage — per-publisher landing page for MCP servers.
 *
 * Route: /mcp/:namespace
 *
 * Distinct from /publishers/:slug (which shows both MCP + agent grids): this
 * page is MCP-only and acts as a scoped entry point for the publisher's MCP
 * catalogue. Under the hood it calls the existing `namespace=X` server-side
 * filter on `GET /api/v1/mcp/servers` — no new endpoint needed.
 *
 * Loading / empty / 404 are three distinct states:
 *   - loading: either query is in flight → render skeleton
 *   - 404:     publisher does not exist → the namespace itself is invalid
 *   - empty:   publisher exists but has no MCP servers → friendly empty state
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
import { ServerCard } from '@/components/mcp/server-card'
import { getPublicClient } from '@/lib/api-client'

export default function MCPNamespacePage() {
  const { namespace } = useParams<{ namespace: string }>()
  const api = getPublicClient()

  // Parallel fetch: publisher metadata drives the page header + 404 state,
  // the server list drives the grid. React Query dedupes both across the
  // site so navigating here from a publisher profile reuses cached data.
  // Normalize the `undefined` data field openapi-fetch returns on a non-2xx
  // response to `null` — react-query explicitly rejects `undefined` as a
  // query result and will log a warning otherwise.
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
    queryKey: ['mcp-servers-ns', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/mcp/servers', {
          params: { query: { namespace: namespace!, limit: 50 } },
        })
        .then((r) => r.data ?? null),
    enabled: !!namespace,
  })

  // Render skeleton while either query is still resolving — the header and
  // list share the same visual frame so partial rendering would flash.
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

  // Publisher not found → the namespace segment in the URL is invalid.
  // `openapi-fetch` resolves non-2xx responses to `{ data: undefined }`
  // instead of throwing, so `!publisher` is the canonical 404 signal here.
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
                <Link to="/mcp">Browse all MCP servers</Link>
              </Button>
            }
          />
        </main>
        <Footer />
      </div>
    )
  }

  const servers = listData?.items ?? []
  const totalCount = listData?.total_count ?? servers.length

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'MCP Servers', href: '/mcp' },
            { label: publisher.slug },
          ]}
        />

        {/* Publisher header — compact, header-row layout. Deliberately
            lighter than /publishers/:slug since this page is MCP-scoped
            rather than a full publisher profile. */}
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
            MCP servers published by{' '}
            <span className="font-mono">{publisher.slug}</span>.
          </p>
        </div>

        {servers.length === 0 ? (
          <EmptyState
            icon={<ResourceIcon type="mcp-server" className="h-10 w-10" />}
            title="No MCP servers yet"
            description={`${publisher.name} hasn't published any MCP servers yet. Check back soon, or browse other publishers.`}
            action={
              <Button variant="outline" size="sm" asChild>
                <Link to="/mcp">Browse all MCP servers</Link>
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-sm text-muted-foreground">
              Showing {servers.length}
              {totalCount > servers.length ? ` of ${totalCount}` : ''} server
              {servers.length !== 1 ? 's' : ''}
            </p>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {servers.map((s) => (
                <ServerCard key={s.id} server={s} />
              ))}
            </div>
          </>
        )}
      </main>
      <Footer />
    </div>
  )
}
