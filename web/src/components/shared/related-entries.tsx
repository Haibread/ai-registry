/**
 * RelatedEntries — shows other entries from the same publisher.
 *
 * Queries by namespace, excludes the current entry by slug, and shows up to 3 cards.
 */

import { useQuery } from '@tanstack/react-query'
import { ServerCard } from '@/components/mcp/server-card'
import { AgentCard } from '@/components/agents/agent-card'
import { getPublicClient } from '@/lib/api-client'

interface RelatedEntriesProps {
  type: 'mcp' | 'agent'
  namespace: string
  currentSlug: string
}

export function RelatedEntries({ type, namespace, currentSlug }: RelatedEntriesProps) {
  const api = getPublicClient()

  const { data } = useQuery({
    queryKey: ['related', type, namespace, currentSlug],
    queryFn: async () => {
      if (type === 'mcp') {
        const r = await api.GET('/api/v1/mcp/servers', {
          params: { query: { namespace, limit: 4 } },
        })
        return r.data?.items?.filter((s) => s.slug !== currentSlug).slice(0, 3) ?? []
      }
      const r = await api.GET('/api/v1/agents', {
        params: { query: { namespace, limit: 4 } },
      })
      return r.data?.items?.filter((a) => a.slug !== currentSlug).slice(0, 3) ?? []
    },
  })

  if (!data || data.length === 0) return null

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-muted-foreground">
        More from {namespace}
      </h3>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {data.map((entry) =>
          type === 'mcp' ? (
            <ServerCard key={entry.id} server={entry as any} />
          ) : (
            <AgentCard key={entry.id} agent={entry as any} />
          ),
        )}
      </div>
    </div>
  )
}
