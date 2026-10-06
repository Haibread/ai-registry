import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowRight, TrendingUp } from 'lucide-react'
import { CardGridSkeleton } from '@/components/ui/card-grid-skeleton'
import { SearchBar } from '@/components/ui/search-bar'
import { SegmentedControl, type SegmentedOption } from '@/components/ui/segmented-control'
import { ProtocolTile } from '@/components/home/protocol-tile'
import { LogoMark } from '@/components/layout/logo'
import { ServerCard } from '@/components/mcp/server-card'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { getPublicClient } from '@/lib/api-client'

type ListingView = 'featured' | 'updated'

const LISTING_OPTIONS: SegmentedOption<ListingView>[] = [
  { value: 'featured', label: 'Featured' },
  { value: 'updated', label: 'Recently updated' },
]

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

  // Recent fallback — only fetch if no featured entries
  const hasFeaturedMcp = (featuredMcp?.items?.length ?? 0) > 0

  const { data: recentMcp } = useQuery({
    queryKey: ['mcp-servers', 'recent'],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { limit: 6 } },
    }).then(r => r.data),
    enabled: listingView === 'featured' && !hasFeaturedMcp && featuredMcp !== undefined,
  })

  // ── Recently updated view queries ──────────────────────────────────
  const { data: updatedMcp } = useQuery({
    queryKey: ['mcp-servers', 'updated'],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
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
  const mcpLabel = listingView === 'updated'
    ? 'Recently Updated MCP Servers'
    : hasFeaturedMcp ? 'Featured MCP Servers' : 'Recent MCP Servers'
  const isLoadingMcp = listingView === 'updated'
    ? updatedMcp === undefined
    : featuredMcp === undefined

  const newThisWeek =
    (stats?.new_mcp_servers_this_week ?? 0) +
    (stats?.new_publishers_this_week ?? 0)
  const statItems = [
    { label: 'MCP servers', value: stats?.mcp_servers },
    { label: 'publishers', value: stats?.publishers },
  ]

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1">
        <section className="border-b bg-muted/40">
          <div className="container grid items-center gap-10 py-12 md:py-16 lg:grid-cols-[minmax(0,1fr)_auto]">
            <div className="space-y-6">
              <h1 className="max-w-2xl text-3xl font-bold tracking-tight sm:text-4xl lg:text-5xl lg:leading-[1.1]">
                Discover, publish and integrate MCP servers.
              </h1>
              <p className="max-w-xl text-lg text-muted-foreground">
                One catalog for the tools your models can call, with versions, publishers
                and ready-to-paste client config.
              </p>
              <SearchBar />
              <dl className="flex flex-wrap items-baseline gap-x-6 gap-y-2 text-sm text-muted-foreground">
                {statItems.map((s) => (
                  <div key={s.label} className="flex items-baseline gap-1.5">
                    <dt className="sr-only">{s.label}</dt>
                    <dd className="text-base font-semibold text-foreground tabular-nums">{s.value ?? '—'}</dd>
                    <span aria-hidden="true">{s.label}</span>
                  </div>
                ))}
                {newThisWeek > 0 && (
                  <div className="flex items-center gap-1 font-medium text-success">
                    <dt className="sr-only">New this week</dt>
                    <TrendingUp className="h-3.5 w-3.5" aria-hidden="true" />
                    <dd>+{newThisWeek} this week</dd>
                  </div>
                )}
              </dl>
            </div>
            <LogoMark className="hidden h-auto w-72 lg:block xl:w-80" />
          </div>
        </section>

        <section className="container pt-10">
          <ProtocolTile mcpCount={stats?.mcp_servers} />
        </section>

        <section className="container space-y-10 py-10">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-2xl font-bold tracking-tight">
              {listingView === 'updated' ? 'Recently updated' : 'Featured'}
            </h2>
            <SegmentedControl
              label="Listing"
              options={LISTING_OPTIONS}
              value={listingView}
              onChange={setListingView}
            />
          </div>

          <div className="space-y-4">
            <div className="flex items-baseline justify-between gap-4">
              <h3 className="font-semibold text-muted-foreground">{mcpLabel}</h3>
              <Link to="/mcp" className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline">
                View all <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
            </div>
            {isLoadingMcp ? (
              <CardGridSkeleton count={3} />
            ) : mcpServers.length > 0 ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {mcpServers.map((s) => <ServerCard key={s.id} server={s} />)}
              </div>
            ) : (
              <p className="py-8 text-center text-sm text-muted-foreground">No MCP servers published yet.</p>
            )}
          </div>
        </section>
      </main>
      <Footer />
    </div>
  )
}
