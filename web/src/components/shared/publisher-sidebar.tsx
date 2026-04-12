/**
 * PublisherSidebar — compact publisher info card shown on detail pages.
 *
 * Shows publisher name, verified status, entry counts, and a link to
 * the full publisher profile.
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
      <div className="rounded-lg border p-4 space-y-2">
        <Skeleton className="h-4 w-24 rounded" />
        <Skeleton className="h-3 w-16 rounded" />
        <Skeleton className="h-3 w-32 rounded" />
      </div>
    )
  }

  return (
    <div className="rounded-lg border p-4 space-y-3">
      <div className="flex items-center gap-2">
        <ResourceIcon type="publisher" className="h-4 w-4 text-muted-foreground" />
        <Link
          to={`/publishers/${namespace}`}
          className="text-sm font-semibold hover:underline"
        >
          {publisher.name}
        </Link>
        {publisher.verified && (
          <Badge variant="default" className="text-[10px] px-1.5 py-0 flex items-center gap-0.5">
            <CheckCircle className="h-2.5 w-2.5" /> Verified
          </Badge>
        )}
      </div>

      <div className="flex gap-4 text-xs text-muted-foreground">
        {mcpCount != null && (
          <span>{mcpCount} MCP server{mcpCount !== 1 ? 's' : ''}</span>
        )}
        {agentCount != null && (
          <span>{agentCount} agent{agentCount !== 1 ? 's' : ''}</span>
        )}
      </div>

      <Link
        to={`/publishers/${namespace}`}
        className="text-xs text-primary hover:underline inline-block"
      >
        View all entries →
      </Link>
    </div>
  )
}
