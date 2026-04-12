import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { History, Package } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { getPublicClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'

function detailHref(resourceType: string, namespace: string, slug: string) {
  if (resourceType === 'mcp_server') return `/mcp/${namespace}/${slug}`
  if (resourceType === 'agent') return `/agents/${namespace}/${slug}`
  return '#'
}

/**
 * Group changelog entries by day (YYYY-MM-DD) so the page renders as a
 * timeline with date headers. Returns entries in insertion order (already
 * sorted newest-first by the API).
 */
function groupByDay<T extends { published_at: string }>(items: T[]): [string, T[]][] {
  const out = new Map<string, T[]>()
  for (const item of items) {
    const day = item.published_at.slice(0, 10)
    if (!out.has(day)) out.set(day, [])
    out.get(day)!.push(item)
  }
  return Array.from(out.entries())
}

export default function ChangelogPage() {
  const api = getPublicClient()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['changelog'],
    queryFn: () =>
      api.GET('/api/v1/changelog', { params: { query: { limit: 100 } } }).then((r) => r.data),
  })

  const items = data?.items ?? []
  const groups = groupByDay(items)

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Breadcrumbs segments={[{ label: 'Home', href: '/' }, { label: 'Changelog' }]} />

        <div className="flex items-center gap-3">
          <History className="h-7 w-7 text-muted-foreground" />
          <h1 className="text-3xl font-bold">Changelog</h1>
        </div>
        <p className="text-muted-foreground">
          Recent version publications across MCP servers and AI agents in the registry.
        </p>

        {isLoading ? (
          <div className="space-y-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded" />
            ))}
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive">Failed to load the changelog.</p>
        ) : groups.length === 0 ? (
          <EmptyState
            icon={<Package className="h-10 w-10" />}
            title="No recent releases"
            description="No versions have been published yet."
          />
        ) : (
          <div className="space-y-8">
            {groups.map(([day, entries]) => (
              <section key={day} className="space-y-2">
                <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {formatDate(day)}
                </h2>
                <ul className="divide-y rounded-md border">
                  {entries.map((e, i) => (
                    <li key={`${e.namespace}/${e.slug}/${e.version}-${i}`} className="flex items-center gap-3 p-3">
                      <ResourceIcon
                        type={e.resource_type === 'mcp_server' ? 'mcp-server' : 'agent'}
                        className="h-5 w-5 shrink-0 text-muted-foreground"
                      />
                      <div className="min-w-0 flex-1">
                        <Link
                          to={detailHref(e.resource_type, e.namespace, e.slug)}
                          className="font-medium hover:underline truncate block"
                        >
                          {e.name}
                        </Link>
                        <p className="text-xs text-muted-foreground font-mono truncate">
                          {e.namespace}/{e.slug}
                        </p>
                      </div>
                      <Badge variant="outline" className="font-mono text-xs shrink-0">
                        v{e.version}
                      </Badge>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}
      </main>
      <Footer />
    </div>
  )
}
