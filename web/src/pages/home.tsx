import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { SearchBar } from '@/components/ui/search-bar'
import { ProtocolExplainer } from '@/components/home/protocol-explainer'
import { ServerListItem } from '@/components/mcp/server-list-item'
import { AgentCard } from '@/components/agents/agent-card'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { getPublicClient } from '@/lib/api-client'

type ListingView = 'featured' | 'updated'

export default function HomePage() {
  const api = getPublicClient()
  const [listingView, setListingView] = useState<ListingView>('featured')

  // ── Featured view queries ──────────────────────────────────────────
  const { data: featuredMcp } = useQuery({
    queryKey: ['mcp-servers', 'featured'],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { featured: true, limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'featured',
  })

  const { data: featuredAgents } = useQuery({
    queryKey: ['agents', 'featured'],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { featured: true, limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'featured',
  })

  // Recent fallback — only fetch if no featured entries
  const hasFeaturedMcp = (featuredMcp?.items?.length ?? 0) > 0
  const hasFeaturedAgents = (featuredAgents?.items?.length ?? 0) > 0

  const { data: recentMcp } = useQuery({
    queryKey: ['mcp-servers', 'recent'],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'featured' && !hasFeaturedMcp && featuredMcp !== undefined,
  })

  const { data: recentAgents } = useQuery({
    queryKey: ['agents', 'recent'],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'featured' && !hasFeaturedAgents && featuredAgents !== undefined,
  })

  // ── Recently updated view queries ──────────────────────────────────
  const { data: updatedMcp } = useQuery({
    queryKey: ['mcp-servers', 'updated'],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { sort: 'updated_at_desc', limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'updated',
  })

  const { data: updatedAgents } = useQuery({
    queryKey: ['agents', 'updated'],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { sort: 'updated_at_desc', limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'updated',
  })

  // Public stats
  const { data: stats } = useQuery({
    queryKey: ['public-stats'],
    queryFn: () => api.GET('/api/v1/public-stats').then(r => r.data),
  })

  // Resolve which data to show based on the current view
  const mcpServers = listingView === 'updated'
    ? (updatedMcp?.items ?? [])
    : hasFeaturedMcp ? featuredMcp!.items! : (recentMcp?.items ?? [])
  const agents = listingView === 'updated'
    ? (updatedAgents?.items ?? [])
    : hasFeaturedAgents ? featuredAgents!.items! : (recentAgents?.items ?? [])
  const mcpLabel = listingView === 'updated'
    ? 'Recently updated MCP servers'
    : hasFeaturedMcp ? 'Featured MCP servers' : 'Recent MCP servers'
  const agentLabel = listingView === 'updated'
    ? 'Recently updated agents'
    : hasFeaturedAgents ? 'Featured agents' : 'Recent agents'
  const isLoadingMcp = listingView === 'updated'
    ? updatedMcp === undefined
    : featuredMcp === undefined
  const isLoadingAgents = listingView === 'updated'
    ? updatedAgents === undefined
    : featuredAgents === undefined

  const statParts: string[] = []
  if (stats?.mcp_servers != null) statParts.push(`${stats.mcp_servers.toLocaleString()} MCP servers`)
  if (stats?.agents != null) statParts.push(`${stats.agents.toLocaleString()} agents`)
  if (stats?.publishers != null) statParts.push(`${stats.publishers.toLocaleString()} publishers`)

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">
        {/* Hero — editorial, left-aligned, inline stats */}
        <section className="border-b">
          <div className="container max-w-4xl py-12 md:py-16">
            <h1 className="text-3xl md:text-4xl font-semibold tracking-tight">
              AI Registry
            </h1>
            <p className="mt-3 max-w-2xl text-base md:text-lg text-muted-foreground">
              A catalog of Model Context Protocol servers and A2A agents. Browse
              what's published, read the cards, and wire things up.
            </p>
            {statParts.length > 0 ? (
              <p className="mt-4 text-sm text-muted-foreground">
                {statParts.join(' · ')}
              </p>
            ) : (
              <p className="mt-4 text-sm text-muted-foreground">&nbsp;</p>
            )}
            <div className="mt-6 max-w-xl">
              <SearchBar />
            </div>
          </div>
        </section>

        {/* Protocol explainer */}
        <section className="border-b">
          <div className="container max-w-4xl py-6">
            <ProtocolExplainer />
          </div>
        </section>

        {/* View toggle */}
        <div className="container max-w-4xl flex justify-end pt-8 pb-2">
          <div className="inline-flex items-center gap-1 rounded-md border p-0.5 text-xs">
            <button
              type="button"
              onClick={() => setListingView('featured')}
              className={`rounded px-2.5 py-1 font-medium transition-colors ${
                listingView === 'featured'
                  ? 'bg-foreground text-background'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              Featured
            </button>
            <button
              type="button"
              onClick={() => setListingView('updated')}
              className={`rounded px-2.5 py-1 font-medium transition-colors ${
                listingView === 'updated'
                  ? 'bg-foreground text-background'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              Recently updated
            </button>
          </div>
        </div>

        {/* MCP Servers — dense list */}
        <section className="py-6 border-b">
          <div className="container max-w-4xl">
            <div className="flex items-baseline justify-between mb-2">
              <h2 className="text-lg font-semibold tracking-tight">{mcpLabel}</h2>
              <Button variant="ghost" size="sm" asChild>
                <Link to="/mcp" className="flex items-center gap-1">View all <ArrowRight className="h-4 w-4" /></Link>
              </Button>
            </div>
            {isLoadingMcp ? (
              <div className="space-y-4 py-4">
                {Array.from({ length: 4 }).map((_, i) => (
                  <div key={i} className="h-14 animate-pulse rounded bg-muted/50" />
                ))}
              </div>
            ) : mcpServers.length > 0 ? (
              <div className="divide-y">
                {mcpServers.map((s) => <ServerListItem key={s.id} server={s} />)}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground py-8">No MCP servers published yet.</p>
            )}
          </div>
        </section>

        {/* Agents — cards for visual contrast */}
        <section className="py-10">
          <div className="container max-w-6xl">
            <div className="flex items-baseline justify-between mb-4">
              <h2 className="text-lg font-semibold tracking-tight">{agentLabel}</h2>
              <Button variant="ghost" size="sm" asChild>
                <Link to="/agents" className="flex items-center gap-1">View all <ArrowRight className="h-4 w-4" /></Link>
              </Button>
            </div>
            {isLoadingAgents ? (
              <CardGridSkeleton count={6} />
            ) : agents.length > 0 ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {agents.map((a) => <AgentCard key={a.id} agent={a} />)}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground py-8">No agents published yet.</p>
            )}
          </div>
        </section>
      </main>
      <Footer />
    </div>
  )
}
