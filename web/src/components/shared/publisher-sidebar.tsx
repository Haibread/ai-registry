/**
 * PublisherSidebar — horizontal publisher banner shown on detail pages.
 *
 * A compact row-style card that identifies who published the entry, shows
 * their verified state, how many MCP servers and agents they maintain, and
 * links to their full profile. Designed to be placed full-width at the top
 * of the Overview tab so it reads as page-level context rather than a
 * sidebar competing with the main metadata.
 */

import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { CheckCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
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

  const { data: agentCount } = useQuery({
    queryKey: ['publisher-agent-count', namespace],
    queryFn: () =>
      api
        .GET('/api/v1/agents', {
          params: { query: { namespace, limit: 1 } },
        })
        .then((r) => r.data?.total_count ?? 0),
  })

  if (!publisher) {
    return (
      <div className="flex items-center gap-3 rounded-lg border bg-card/40 px-4 py-3">
        <Skeleton className="h-5 w-5 rounded" />
        <Skeleton className="h-4 w-32 rounded" />
        <Skeleton className="h-3 w-40 rounded" />
      </div>
    )
  }

  return (
    <div className="flex items-center gap-x-4 gap-y-2 rounded-lg border bg-card/40 px-4 py-3 flex-wrap">
      <div className="flex items-center gap-2 min-w-0">
        <ResourceIcon type="publisher" className="h-4 w-4 text-muted-foreground shrink-0" />
        <span className="text-xs text-muted-foreground">Published by</span>
        <Link
          to={`/publishers/${namespace}`}
          className="text-sm font-semibold hover:underline truncate"
        >
          {publisher.name}
        </Link>
        {publisher.verified && (
          <Badge variant="default" className="text-[10px] px-1.5 py-0 flex items-center gap-0.5">
            <CheckCircle className="h-2.5 w-2.5" /> Verified
          </Badge>
        )}
      </div>

      <span className="hidden sm:inline h-4 w-px bg-border" aria-hidden="true" />

      <div className="flex items-center gap-4 text-xs text-muted-foreground">
        {mcpCount != null && (
          <span>
            <span className="font-semibold text-foreground tabular-nums">{mcpCount}</span>{' '}
            MCP server{mcpCount !== 1 ? 's' : ''}
          </span>
        )}
        {agentCount != null && (
          <span>
            <span className="font-semibold text-foreground tabular-nums">{agentCount}</span>{' '}
            agent{agentCount !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      <Link
        to={`/publishers/${namespace}`}
        className="ml-auto text-xs text-primary hover:underline"
      >
        View all entries →
      </Link>
    </div>
  )
}
