import { Link } from 'react-router-dom'
import { VerifiedBadge, StatusBadge } from '@/components/ui/badge'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

interface ServerListItemProps {
  server: MCPServer
}

export function ServerListItem({ server }: ServerListItemProps) {
  const lv = server.latest_version
  const to = `/mcp/${server.namespace}/${server.slug}`
  const ecosystem = lv?.packages?.[0] ? ecosystemLabel(lv.packages[0].registryType) : null
  const hasRemote = lv?.packages?.some((p) => isRemoteTransport(p.transport.type)) ?? false

  return (
    <Link
      to={to}
      className="group grid grid-cols-[1fr_auto] items-baseline gap-x-6 gap-y-1 py-4 border-b last:border-b-0 hover:bg-muted/30 -mx-2 px-2 transition-colors"
    >
      <div className="min-w-0">
        <div className="flex items-baseline gap-2 flex-wrap">
          <h3 className="text-base font-semibold text-foreground group-hover:text-primary transition-colors">
            {server.name}
          </h3>
          <span className="text-xs font-mono text-muted-foreground">
            {server.namespace}/{server.slug}
          </span>
          {lv && (
            <span className="text-xs font-mono text-muted-foreground">
              v{lv.version}
            </span>
          )}
          {server.verified && <VerifiedBadge className="text-[10px]" />}
          {server.status !== 'published' && (
            <StatusBadge status={server.status} className="text-[10px]" />
          )}
        </div>
        {server.description && (
          <p className="mt-1 text-sm text-muted-foreground line-clamp-1">
            {server.description}
          </p>
        )}
      </div>
      <div className="flex flex-col items-end gap-1 text-xs text-muted-foreground shrink-0">
        <div className="flex items-center gap-2">
          {lv?.runtime && <span className="font-mono">{lv.runtime}</span>}
          {hasRemote && (
            <span className="text-[10px] uppercase tracking-wide text-muted-foreground/70">
              remote
            </span>
          )}
          {ecosystem && <span>{ecosystem}</span>}
        </div>
        <FreshnessIndicator updatedAt={server.updated_at} />
      </div>
    </Link>
  )
}
