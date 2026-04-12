/**
 * PublisherDetailPage — public profile page for a publisher.
 *
 * Shows publisher info + grids of their MCP servers and agents.
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
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { ServerCard } from '@/components/mcp/server-card'
import { AgentCard } from '@/components/agents/agent-card'
import { getPublicClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'

export default function PublisherDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  const api = getPublicClient()

  const { data: publisher, isLoading, isError } = useQuery({
    queryKey: ['publisher', slug],
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{slug}', {
          params: { path: { slug: slug! } },
        })
        .then((r) => r.data),
    enabled: !!slug,
  })

  const { data: mcpData } = useQuery({
    queryKey: ['publisher-mcp', slug],
    queryFn: () =>
      api
        .GET('/api/v1/mcp/servers', {
          params: { query: { namespace: slug!, limit: 20 } },
        })
        .then((r) => r.data),
    enabled: !!slug,
  })

  const { data: agentData } = useQuery({
    queryKey: ['publisher-agents', slug],
    queryFn: () =>
      api
        .GET('/api/v1/agents', {
          params: { query: { namespace: slug!, limit: 20 } },
        })
        .then((r) => r.data),
    enabled: !!slug,
  })

  if (isLoading)
    return (
      <div className="flex min-h-screen flex-col">
        <Header />
        <main className="flex-1 container py-8 max-w-4xl">
          <DetailPageSkeleton />
        </main>
        <Footer />
      </div>
    )

  if (isError || !publisher)
    return (
      <div className="flex min-h-screen flex-col">
        <Header />
        <main className="flex-1 container py-8 max-w-4xl">
          <EmptyState
            icon={<ResourceIcon type="publisher" className="h-10 w-10" />}
            title="Publisher not found"
            description="This publisher does not exist."
            action={
              <Button variant="outline" size="sm" asChild>
                <Link to="/">Back to Home</Link>
              </Button>
            }
          />
        </main>
        <Footer />
      </div>
    )

  const mcpServers = mcpData?.items ?? []
  const agents = agentData?.items ?? []

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-4xl space-y-8">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'Publishers' },
            { label: publisher.slug },
          ]}
        />

        {/* Publisher info */}
        <div className="space-y-3">
          <div className="flex items-center gap-3 flex-wrap">
            <ResourceIcon type="publisher" className="h-8 w-8 text-muted-foreground" />
            <h1 className="text-2xl font-bold">{publisher.name}</h1>
            {publisher.verified && (
              <Badge variant="default" className="flex items-center gap-1">
                <CheckCircle className="h-3 w-3" /> Verified
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground font-mono">{publisher.slug}</p>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm max-w-md">
            {publisher.contact && (
              <>
                <dt className="text-muted-foreground">Contact</dt>
                <dd>{publisher.contact}</dd>
              </>
            )}
            <dt className="text-muted-foreground">Joined</dt>
            <dd>{formatDate(publisher.created_at)}</dd>
          </dl>
        </div>

        {/* MCP Servers section */}
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <ResourceIcon type="mcp-server" className="h-4 w-4" />
              MCP Servers
              {mcpData?.total_count != null && (
                <span className="text-sm font-normal text-muted-foreground">
                  ({mcpData.total_count})
                </span>
              )}
            </h2>
            {mcpServers.length > 0 && (
              <Button variant="ghost" size="sm" asChild>
                <Link to={`/mcp?namespace=${slug}`}>View all →</Link>
              </Button>
            )}
          </div>
          {mcpData === undefined ? (
            <CardGridSkeleton count={3} />
          ) : mcpServers.length > 0 ? (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {mcpServers.map((s) => (
                <ServerCard key={s.id} server={s} />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground py-4">
              No MCP servers published yet.
            </p>
          )}
        </section>

        {/* Agents section */}
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <ResourceIcon type="agent" className="h-4 w-4" />
              Agents
              {agentData?.total_count != null && (
                <span className="text-sm font-normal text-muted-foreground">
                  ({agentData.total_count})
                </span>
              )}
            </h2>
            {agents.length > 0 && (
              <Button variant="ghost" size="sm" asChild>
                <Link to={`/agents?namespace=${slug}`}>View all →</Link>
              </Button>
            )}
          </div>
          {agentData === undefined ? (
            <CardGridSkeleton count={3} />
          ) : agents.length > 0 ? (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {agents.map((a) => (
                <AgentCard key={a.id} agent={a} />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground py-4">
              No agents published yet.
            </p>
          )}
        </section>
      </main>
      <Footer />
    </div>
  )
}
