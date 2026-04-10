import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Server, Bot, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ServerCard } from '@/components/mcp/server-card'
import { AgentCard } from '@/components/agents/agent-card'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { getPublicClient } from '@/lib/api-client'

export default function HomePage() {
  const api = getPublicClient()

  const { data: mcpData } = useQuery({
    queryKey: ['mcp-servers', 'home'],
    queryFn: () => api.GET('/api/v1/mcp/servers', { params: { query: { limit: 6 } } }).then(r => r.data),
  })

  const { data: agentsData } = useQuery({
    queryKey: ['agents', 'home'],
    queryFn: () => api.GET('/api/v1/agents', { params: { query: { limit: 6 } } }).then(r => r.data),
  })

  const mcpServers = mcpData?.items ?? []
  const agents = agentsData?.items ?? []
  const mcpTotal = mcpData?.total_count ?? mcpServers.length
  const agentsTotal = agentsData?.total_count ?? agents.length

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">
        {/* Hero */}
        <section className="border-b bg-muted/30 py-16">
          <div className="container text-center space-y-4">
            <h1 className="text-4xl font-bold tracking-tight">AI Registry</h1>
            <p className="text-lg text-muted-foreground max-w-xl mx-auto">
              A centralized catalog of MCP servers and AI agents. Discover, publish, and integrate.
            </p>
            <div className="flex justify-center gap-3 pt-2">
              <Button asChild><Link to="/mcp">Browse MCP Servers</Link></Button>
              <Button variant="outline" asChild><Link to="/agents">Browse Agents</Link></Button>
            </div>
          </div>
        </section>

        {/* Stats */}
        <section className="border-b py-8">
          <div className="container grid grid-cols-2 gap-4 max-w-sm mx-auto">
            <Link to="/mcp" className="group">
              <Card className="transition-shadow hover:shadow-md">
                <CardHeader className="pt-4 pb-1">
                  <CardTitle className="text-sm font-medium text-muted-foreground flex items-center justify-center gap-2 group-hover:text-primary transition-colors">
                    <Server className="h-4 w-4" aria-hidden="true" /> MCP Servers
                  </CardTitle>
                </CardHeader>
                <CardContent className="pb-4">
                  <p className="text-3xl font-bold text-center">{mcpTotal}</p>
                </CardContent>
              </Card>
            </Link>
            <Link to="/agents" className="group">
              <Card className="transition-shadow hover:shadow-md">
                <CardHeader className="pt-4 pb-1">
                  <CardTitle className="text-sm font-medium text-muted-foreground flex items-center justify-center gap-2 group-hover:text-primary transition-colors">
                    <Bot className="h-4 w-4" aria-hidden="true" /> Agents
                  </CardTitle>
                </CardHeader>
                <CardContent className="pb-4">
                  <p className="text-3xl font-bold text-center">{agentsTotal}</p>
                </CardContent>
              </Card>
            </Link>
          </div>
        </section>

        {mcpServers.length > 0 && (
          <section className="py-10 border-b">
            <div className="container">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-semibold">Recent MCP Servers</h2>
                <Button variant="ghost" size="sm" asChild>
                  <Link to="/mcp" className="flex items-center gap-1">View all <ArrowRight className="h-4 w-4" /></Link>
                </Button>
              </div>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {mcpServers.map((s) => <ServerCard key={s.id} server={s} />)}
              </div>
            </div>
          </section>
        )}

        {agents.length > 0 && (
          <section className="py-10">
            <div className="container">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-semibold">Recent Agents</h2>
                <Button variant="ghost" size="sm" asChild>
                  <Link to="/agents" className="flex items-center gap-1">View all <ArrowRight className="h-4 w-4" /></Link>
                </Button>
              </div>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {agents.map((a) => <AgentCard key={a.id} agent={a} />)}
              </div>
            </div>
          </section>
        )}
      </main>
      <Footer />
    </div>
  )
}
