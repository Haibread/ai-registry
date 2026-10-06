/**
 * PublisherSidebar — the publisher card in a detail page's side column:
 * who published the entry, whether they are verified, how many entries they
 * maintain, and a link to their profile.
 */

import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowRight, BadgeCheck } from 'lucide-react'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { Skeleton } from '@/components/ui/skeleton'
import { getPublicClient } from '@/lib/api-client'

interface PublisherSidebarProps {
  namespace: string
}

export function PublisherSidebar({ namespace }: PublisherSidebarProps) {
  const api = getPublicClient()

  const { data: publisher } = useQuery({
    queryKey: ['publisher', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{slug}', {
          params: { path: { slug: namespace } },
        })
        .then((r) => r.data),
  })

  // Quick count queries (limit=0 would be ideal but limit=1 works too)
  const { data: mcpCount } = useQuery({
    queryKey: ['publisher-mcp-count', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/mcp/servers', {
          params: { query: { namespace, limit: 1 } },
        })
        .then((r) => r.data?.total_count ?? 0),
  })

  if (!publisher) {
    return (
      <div className="flex items-center gap-3 rounded-xl border bg-card p-4">
        <Skeleton className="h-10 w-10 rounded-lg" />
        <div className="space-y-1.5">
          <Skeleton className="h-4 w-28 rounded" />
          <Skeleton className="h-3 w-36 rounded" />
        </div>
      </div>
    )
  }

  return (
    <div className="relative flex items-center gap-3 rounded-xl border bg-card p-4 transition-colors hover:border-primary/40">
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-secondary text-secondary-foreground"
        aria-hidden="true"
      >
        <ResourceIcon type="publisher" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-xs text-muted-foreground">Published by</p>
        <div className="flex items-center gap-1.5">
          <Link
            to={`/publishers/${namespace}`}
            className="truncate font-semibold hover:text-primary after:absolute after:inset-0 after:content-['']"
          >
            {publisher.name}
          </Link>
          {publisher.verified && (
            <span className="inline-flex shrink-0 items-center gap-0.5 text-xs font-medium text-primary">
              <BadgeCheck className="h-3.5 w-3.5" aria-hidden="true" /> Verified
            </span>
          )}
        </div>
        {mcpCount != null && (
          <p className="text-xs text-muted-foreground">
            {mcpCount} MCP server{mcpCount !== 1 ? 's' : ''}
          </p>
        )}
      </div>
      <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
    </div>
  )
}
