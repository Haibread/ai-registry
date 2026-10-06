/**
 * RelatedEntries — shows other MCP servers from the same publisher.
 *
 * Queries by namespace, excludes the current entry by slug, and shows up to 3 cards.
 */

import { useQuery } from '@tanstack/react-query'
import { ServerCard } from '@/components/mcp/server-card'
import { getPublicClient } from '@/lib/api-client'

interface RelatedEntriesProps {
  namespace: string
  currentSlug: string
}

export function RelatedEntries({ namespace, currentSlug }: RelatedEntriesProps) {
  const api = getPublicClient()

  const { data } = useQuery({
    queryKey: ['related', namespace, currentSlug],
    queryFn: async () => {
      const r = await api.GET('/api/v1/mcp/servers', {
        params: { query: { namespace, limit: 4 } },
      })
      return r.data?.items?.filter((s) => s.slug !== currentSlug).slice(0, 3) ?? []
    },
  })

  if (!data || data.length === 0) return null

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-muted-foreground">
        More from {namespace}
      </h3>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {data.map((entry) => (
          <ServerCard key={entry.id} server={entry} />
        ))}
      </div>
    </div>
  )
}
