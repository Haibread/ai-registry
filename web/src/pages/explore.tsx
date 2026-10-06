/**
 * ExplorePage — search + browse across MCP servers.
 */

import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { Search } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { ServerCard } from '@/components/mcp/server-card'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

export default function ExplorePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? ''
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

  const listQuery = {
    q: q || undefined,
    limit: 20,
    sort: sort as 'created_at_desc' | 'updated_at_desc' | 'published_at_desc' | 'name_asc' | 'name_desc' | undefined,
  }

  const { data: mcpData, isLoading: mcpLoading } = useQuery({
    queryKey: ['explore-mcp', listQuery],
    queryFn: () =>
      api.GET('/api/v1/mcp/servers', { params: { query: listQuery } }).then((r) => r.data),
  })

  const mcpServers = mcpData?.items ?? []

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
            Search and browse across MCP servers.
          </p>
        </div>

        {/* Search bar */}
        <form onSubmit={handleSearch} className="flex gap-2 max-w-lg">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
            <Input
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              placeholder="Search MCP servers..."
              className="pl-10"
              aria-label="Search explore"
            />
          </div>
          <Button type="submit">Search</Button>
        </form>

        <div className="flex flex-wrap items-center gap-3">
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
                setSearchParams({}, { replace: true })
              }}
            >
              Clear filters
            </Button>
          )}
        </div>

        {/* Results */}
        {mcpLoading ? (
          <CardGridSkeleton count={6} />
        ) : mcpServers.length === 0 ? (
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
          <section>
            <h2 className="text-lg font-semibold flex items-center gap-2 mb-4">
              <ResourceIcon type="mcp-server" className="h-4 w-4" />
              MCP Servers
              {mcpData?.total_count != null && (
                <span className="text-sm font-normal text-muted-foreground">
                  ({mcpData.total_count})
                </span>
              )}
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {mcpServers.map((s) => (
                <ServerCard key={s.id} server={s} />
              ))}
            </div>
          </section>
        )}
      </main>
      <Footer />
    </div>
  )
}
