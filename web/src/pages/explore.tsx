/**
 * ExplorePage — unified search + browse across MCP servers and agents.
 *
 * Type filter tabs: All / MCP Servers / Agents.
 * Reuses FilterBar with type-specific filters.
 * Fires parallel queries, merges and renders results.
 */

import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Search } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { ServerCard } from '@/components/mcp/server-card'
import { AgentCard } from '@/components/agents/agent-card'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

type TypeTab = 'all' | 'mcp' | 'agents'

const TABS: { value: TypeTab; label: string; iconType?: 'mcp-server' | 'agent' }[] = [
  { value: 'all', label: 'All' },
  { value: 'mcp', label: 'MCP Servers', iconType: 'mcp-server' },
  { value: 'agents', label: 'Agents', iconType: 'agent' },
]

export default function ExplorePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? ''
  const type = (searchParams.get('type') ?? 'all') as TypeTab
  const sort = searchParams.get('sort') ?? undefined

  const [inputValue, setInputValue] = useState(q)

  const api = getPublicClient()

  const setParam = useCallback(
    (key: string, value: string | undefined) => {
      const p = new URLSearchParams(searchParams)
      if (value) p.set(key, value)
      else p.delete(key)
      p.delete('cursor')
      setSearchParams(p, { replace: true })
    },
    [searchParams, setSearchParams],
  )

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setParam('q', inputValue.trim() || undefined)
  }

  const setType = (t: TypeTab) => {
    const p = new URLSearchParams(searchParams)
    if (t === 'all') p.delete('type')
    else p.set('type', t)
    p.delete('cursor')
    setSearchParams(p, { replace: true })
  }

  // Parallel queries — enabled based on type tab
  const showMcp = type === 'all' || type === 'mcp'
  const showAgents = type === 'all' || type === 'agents'

  const { data: mcpData, isLoading: mcpLoading } = useQuery({
    queryKey: ['explore-mcp', { q, sort }],
    queryFn: () =>
      api
        .GET('/api/v1/mcp/servers', {
          params: {
            query: {
              q: q || undefined,
              limit: type === 'all' ? 6 : 20,
              sort: sort as 'created_at_desc' | 'updated_at_desc' | 'published_at_desc' | 'name_asc' | 'name_desc' | undefined,
            },
          },
        })
        .then((r) => r.data),
    enabled: showMcp,
  })

  const { data: agentData, isLoading: agentLoading } = useQuery({
    queryKey: ['explore-agents', { q, sort }],
    queryFn: () =>
      api
        .GET('/api/v1/agents', {
          params: {
            query: {
              q: q || undefined,
              limit: type === 'all' ? 6 : 20,
              sort: sort as 'created_at_desc' | 'updated_at_desc' | 'published_at_desc' | 'name_asc' | 'name_desc' | undefined,
            },
          },
        })
        .then((r) => r.data),
    enabled: showAgents,
  })

  const mcpServers = mcpData?.items ?? []
  const agents = agentData?.items ?? []
  const isLoading = (showMcp && mcpLoading) || (showAgents && agentLoading)
  const hasResults = mcpServers.length > 0 || agents.length > 0

  const sortOptions = [
    { value: 'created_at_desc', label: 'Newest first' },
    { value: 'updated_at_desc', label: 'Recently updated' },
    { value: 'published_at_desc', label: 'Recently published' },
    { value: 'name_asc', label: 'Name A–Z' },
    { value: 'name_desc', label: 'Name Z–A' },
  ]

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold">Explore</h1>
          <p className="text-muted-foreground mt-1">
            Search and browse across MCP servers and AI agents.
          </p>
        </div>

        {/* Search bar */}
        <form onSubmit={handleSearch} className="flex gap-2 max-w-lg">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
            <Input
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              placeholder="Search everything..."
              className="pl-10"
              aria-label="Search explore"
            />
          </div>
          <Button type="submit">Search</Button>
        </form>

        {/* Type tabs + sort */}
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-1 rounded-lg border p-1">
            {TABS.map((tab) => (
              <button
                key={tab.value}
                type="button"
                onClick={() => setType(tab.value)}
                className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                  type === tab.value
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                }`}
              >
                {tab.iconType && <ResourceIcon type={tab.iconType} className="h-3.5 w-3.5" />}
                {tab.label}
              </button>
            ))}
          </div>

          <select
            value={sort ?? ''}
            onChange={(e) => setParam('sort', e.target.value || undefined)}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            aria-label="Sort order"
          >
            <option value="">Sort: Default</option>
            {sortOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>

          {(q || sort) && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setInputValue('')
                setSearchParams(type !== 'all' ? { type } : {}, { replace: true })
              }}
            >
              Clear filters
            </Button>
          )}
        </div>

        {/* Results */}
        {isLoading ? (
          <CardGridSkeleton count={6} />
        ) : !hasResults ? (
          <EmptyState
            icon={<Search className="h-10 w-10" />}
            title={q ? 'No results found' : 'Nothing here yet'}
            description={
              q
                ? `No entries match "${q}". Try a different search term.`
                : 'No entries have been published yet.'
            }
          />
        ) : (
          <div className="space-y-8">
            {/* MCP Servers section */}
            {showMcp && mcpServers.length > 0 && (
              <section>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-lg font-semibold flex items-center gap-2">
                    <ResourceIcon type="mcp-server" className="h-4 w-4" />
                    MCP Servers
                    {mcpData?.total_count != null && (
                      <span className="text-sm font-normal text-muted-foreground">
                        ({mcpData.total_count})
                      </span>
                    )}
                  </h2>
                  {type === 'all' && mcpServers.length >= 6 && (
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/mcp${q ? `?q=${encodeURIComponent(q)}` : ''}`}>
                        View all →
                      </Link>
                    </Button>
                  )}
                </div>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {mcpServers.map((s) => (
                    <ServerCard key={s.id} server={s} />
                  ))}
                </div>
              </section>
            )}

            {/* Agents section */}
            {showAgents && agents.length > 0 && (
              <section>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-lg font-semibold flex items-center gap-2">
                    <ResourceIcon type="agent" className="h-4 w-4" />
                    Agents
                    {agentData?.total_count != null && (
                      <span className="text-sm font-normal text-muted-foreground">
                        ({agentData.total_count})
                      </span>
                    )}
                  </h2>
                  {type === 'all' && agents.length >= 6 && (
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/agents${q ? `?q=${encodeURIComponent(q)}` : ''}`}>
                        View all →
                      </Link>
                    </Button>
                  )}
                </div>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {agents.map((a) => (
                    <AgentCard key={a.id} agent={a} />
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
      </main>
      <Footer />
    </div>
  )
}
