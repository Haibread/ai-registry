import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowRight, Building2, TrendingUp } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { SearchBar } from '@/components/ui/search-bar'
import { ProtocolExplainer } from '@/components/home/protocol-explainer'
import { ServerCard } from '@/components/mcp/server-card'
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
    ? 'Recently Updated MCP Servers'
    : hasFeaturedMcp ? 'Featured MCP Servers' : 'Recent MCP Servers'
  const agentLabel = listingView === 'updated'
    ? 'Recently Updated Agents'
    : hasFeaturedAgents ? 'Featured Agents' : 'Recent Agents'
  const isLoadingMcp = listingView === 'updated'
    ? updatedMcp === undefined
    : featuredMcp === undefined
  const isLoadingAgents = listingView === 'updated'
    ? updatedAgents === undefined
    : featuredAgents === undefined

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">
        {/* Hero */}
        <section className="border-b bg-muted/30 py-16">
          <div className="container text-center space-y-6">
            <h1 className="text-4xl font-bold tracking-tight">AI Registry</h1>
            <p className="text-lg text-muted-foreground max-w-xl mx-auto">
              A centralized catalog of MCP servers and AI agents. Discover, publish, and integrate.
            </p>
            <SearchBar />
          </div>
        </section>

        {/* Stats */}
        <section className="border-b py-8">
          <div className="container grid grid-cols-1 sm:grid-cols-3 gap-4 max-w-2xl mx-auto">
            <Link to="/mcp" className="group">
              <Card className="h-full transition-shadow hover:shadow-md">
                <CardHeader className="pt-4 pb-1">
                  <CardTitle className="text-xs font-medium text-muted-foreground flex items-center justify-center gap-1.5 group-hover:text-primary transition-colors">
                    <ResourceIcon type="mcp-server" className="h-3.5 w-3.5" /> MCP Servers
                  </CardTitle>
                </CardHeader>
                <CardContent className="pb-4">
                  <p className="text-2xl font-bold text-center">{stats?.mcp_servers ?? '—'}</p>
                  <p className="text-[10px] text-center text-green-600 dark:text-green-400 flex items-center justify-center gap-0.5 mt-0.5 min-h-[14px]">
                    {stats?.new_mcp_servers_this_week != null && stats.new_mcp_servers_this_week > 0 && (
                      <>
                        <TrendingUp className="h-2.5 w-2.5" />+{stats.new_mcp_servers_this_week} this week
                      </>
                    )}
                  </p>
                </CardContent>
              </Card>
            </Link>
            <Link to="/agents" className="group">
              <Card className="h-full transition-shadow hover:shadow-md">
                <CardHeader className="pt-4 pb-1">
                  <CardTitle className="text-xs font-medium text-muted-foreground flex items-center justify-center gap-1.5 group-hover:text-primary transition-colors">
                    <ResourceIcon type="agent" className="h-3.5 w-3.5" /> Agents
                  </CardTitle>
                </CardHeader>
                <CardContent className="pb-4">
                  <p className="text-2xl font-bold text-center">{stats?.agents ?? '—'}</p>
                  <p className="text-[10px] text-center text-green-600 dark:text-green-400 flex items-center justify-center gap-0.5 mt-0.5 min-h-[14px]">
                    {stats?.new_agents_this_week != null && stats.new_agents_this_week > 0 && (
                      <>
                        <TrendingUp className="h-2.5 w-2.5" />+{stats.new_agents_this_week} this week
                      </>
                    )}
                  </p>
                </CardContent>
              </Card>
            </Link>
            <Card className="h-full">
              <CardHeader className="pt-4 pb-1">
                <CardTitle className="text-xs font-medium text-muted-foreground flex items-center justify-center gap-1.5">
                  <Building2 className="h-3.5 w-3.5" /> Publishers
                </CardTitle>
              </CardHeader>
              <CardContent className="pb-4">
                <p className="text-2xl font-bold text-center">{stats?.publishers ?? '—'}</p>
                <p className="text-[10px] text-center text-green-600 dark:text-green-400 flex items-center justify-center gap-0.5 mt-0.5 min-h-[14px]">
                  {stats?.new_publishers_this_week != null && stats.new_publishers_this_week > 0 && (
                    <>
                      <TrendingUp className="h-2.5 w-2.5" />+{stats.new_publishers_this_week} this week
                    </>
                  )}
                </p>
              </CardContent>
            </Card>
          </div>
        </section>

        {/* Protocol explainer */}
        <section className="py-6">
          <div className="container max-w-2xl">
            <ProtocolExplainer />
          </div>
        </section>

        {/* View toggle */}
        <div className="container flex justify-center py-2">
          <div className="inline-flex items-center gap-1 rounded-lg border p-1">
            <button
              type="button"
              onClick={() => setListingView('featured')}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                listingView === 'featured'
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              }`}
            >
              Featured / New
            </button>
            <button
              type="button"
              onClick={() => setListingView('updated')}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                listingView === 'updated'
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              }`}
            >
              Recently Updated
            </button>
          </div>
        </div>

        {/* MCP Servers */}
        <section className="py-10 border-b">
          <div className="container">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold">{mcpLabel}</h2>
              <Button variant="ghost" size="sm" asChild>
                <Link to="/mcp" className="flex items-center gap-1">View all <ArrowRight className="h-4 w-4" /></Link>
              </Button>
            </div>
            {isLoadingMcp ? (
              <CardGridSkeleton count={6} />
            ) : mcpServers.length > 0 ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {mcpServers.map((s) => <ServerCard key={s.id} server={s} />)}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground text-center py-8">No MCP servers published yet.</p>
            )}
          </div>
        </section>

        {/* Agents */}
        <section className="py-10">
          <div className="container">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold">{agentLabel}</h2>
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
              <p className="text-sm text-muted-foreground text-center py-8">No agents published yet.</p>
            )}
          </div>
        </section>
      </main>
      <Footer />
    </div>
  )
}
