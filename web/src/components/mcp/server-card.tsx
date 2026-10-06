import { Link } from 'react-router-dom'
import { BadgeCheck, Eye } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Badge, FeaturedBadge, StatusBadge } from '@/components/ui/badge'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { TagBadge } from '@/components/ui/tag-badge'
import { formatCount, ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import { indexInstanceTags, useInstanceTags } from '@/lib/use-instance-tags'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

interface ServerCardProps {
  server: MCPServer
}

const MAX_TAGS = 2

/** How a client reaches the server: the remote transport when it has one,
 *  otherwise the package ecosystem it installs from. */
function connectionLabel(lv: MCPServer['latest_version']): string | null {
  if (!lv) return null
  const remote = lv.remotes?.[0]?.type
    ?? lv.packages?.find((p) => isRemoteTransport(p.transport.type) && p.transport.url)?.transport.type
  if (remote) return `remote · ${remote}`
  const pkg = lv.packages?.[0]
  return pkg ? `local · ${ecosystemLabel(pkg.registryType)}` : null
}

export function ServerCard({ server }: ServerCardProps) {
  const lv = server.latest_version
  const to = `/mcp/${server.namespace}/${server.slug}`
  const { data: tagData } = useInstanceTags()
  const tagIndex = indexInstanceTags(tagData?.items)
  const connection = connectionLabel(lv)
  const toolCount = lv?.tools?.length ?? 0
  const tags = server.tags ?? []
  const views = server.view_count ?? 0

  return (
    <Card className="relative flex flex-col gap-3 p-4 transition-colors hover:border-primary/40 hover:shadow-sm">
      <div className="flex items-start gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-secondary-foreground" aria-hidden="true">
          <ResourceIcon type="mcp-server" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <h3 className="min-w-0 truncate font-semibold leading-snug">
              <Link
                to={to}
                className="hover:text-primary transition-colors after:absolute after:inset-0 after:content-['']"
              >
                {server.name}
              </Link>
            </h3>
            {server.verified && (
              <BadgeCheck className="h-4 w-4 shrink-0 text-primary" aria-label="Verified" role="img" />
            )}
          </div>
          <div className="truncate font-mono text-xs text-muted-foreground">
            <Link to={`/mcp/${server.namespace}`} className="relative z-10 hover:text-foreground transition-colors">
              {server.namespace}
            </Link>
            /{server.slug}
          </div>
        </div>
        <div className="relative z-10 flex shrink-0 flex-col items-end gap-1">
          {server.featured && <FeaturedBadge className="text-[11px]" />}
          {server.status !== 'published' && <StatusBadge status={server.status} className="text-[11px]" />}
        </div>
      </div>

      {server.description && (
        <p className="line-clamp-2 text-sm text-muted-foreground">{server.description}</p>
      )}

      {(connection || toolCount > 0 || tags.length > 0) && (
        <div className="relative z-10 flex flex-wrap gap-1.5">
          {connection && <Badge variant="muted" className="rounded-md font-medium">{connection}</Badge>}
          {toolCount > 0 && (
            <Badge variant="muted" className="rounded-md font-medium">
              {toolCount} tool{toolCount !== 1 ? 's' : ''}
            </Badge>
          )}
          {tags.slice(0, MAX_TAGS).map((slug) => (
            <TagBadge key={slug} slug={slug} tag={tagIndex.get(slug)} />
          ))}
          {tags.length > MAX_TAGS && (
            <Badge variant="outline" className="rounded-md font-normal text-muted-foreground">
              +{tags.length - MAX_TAGS}
            </Badge>
          )}
        </div>
      )}

      <div className="mt-auto flex items-center justify-between gap-2 border-t pt-3 text-xs text-muted-foreground">
        <div className="flex min-w-0 items-center gap-2">
          {lv && <span className="font-mono">v{lv.version}</span>}
          <FreshnessIndicator updatedAt={server.updated_at} />
        </div>
        <span className="inline-flex items-center gap-1" aria-label={`${views.toLocaleString()} views`}>
          <Eye className="h-3 w-3" aria-hidden="true" />
          {formatCount(views)}
        </span>
      </div>
    </Card>
  )
}
