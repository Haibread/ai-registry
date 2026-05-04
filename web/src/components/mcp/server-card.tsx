import { Link } from 'react-router-dom'
import { ExternalLink, Eye, GitFork, Braces, Link2, Cpu, Boxes } from 'lucide-react'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge, StatusBadge, VerifiedBadge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { formatCount, ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

interface ServerCardProps {
  server: MCPServer
}

export function ServerCard({ server }: ServerCardProps) {
  const lv = server.latest_version
  const to = `/mcp/${server.namespace}/${server.slug}`

  // Show at most one ecosystem badge to avoid visual noise
  const ecosystem = lv?.packages?.[0] ? ecosystemLabel(lv.packages[0].registryType) : null

  // For non-stdio transports, surface the transport type + endpoint URL on the card
  const remotePkg = lv?.packages?.find(p => isRemoteTransport(p.transport.type) && p.transport.url) ?? null
  const endpointUrl = remotePkg?.transport.url ?? null
  const transportType = remotePkg?.transport.type ?? null

  // `tools` is a first-class JSONB array on the latest version (see
  // migration 000007). Hide the chip when the field is absent or empty —
  // mirrors agent-card.tsx, which also only renders the skills chip when
  // the count is non-zero.
  const toolCount = lv?.tools?.length ?? 0

  return (
    <Card className="flex flex-col hover:shadow-md transition-shadow group relative">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-base leading-snug flex items-center gap-2 min-w-0">
            <div className="rounded-md bg-primary/10 p-1.5 shrink-0" aria-hidden="true">
              <Boxes className="h-4 w-4 text-primary" />
            </div>
            <Link
              to={to}
              className="truncate hover:text-primary transition-colors after:absolute after:inset-0 after:content-['']"
            >
              {server.name}
            </Link>
          </CardTitle>
          <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end relative z-10">
            {lv && (
              <Badge variant="outline" className="text-[11px] font-mono">
                v{lv.version}
              </Badge>
            )}
            {server.verified && <VerifiedBadge className="text-[10px]" />}
            <StatusBadge status={server.status} className="text-[11px]" />
          </div>
        </div>
        <div className="text-xs text-muted-foreground font-mono relative z-10">
          <Link
            to={`/mcp/${server.namespace}`}
            className="hover:text-foreground transition-colors"
          >
            {server.namespace}
          </Link>
          /{server.slug}
        </div>

        {/* Runtime + one ecosystem chip */}
        {lv && (
          <div className="flex flex-wrap gap-1 pt-1">
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
              {lv.runtime}
            </Badge>
            {ecosystem && (
              <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                {ecosystem}
              </Badge>
            )}
          </div>
        )}

        {/* Tool count + top tool names — mirrors the agent-card skills row,
            where `[Cpu N skills]` sits alongside up to 3 tag badges pulled
            from `skills.flatMap(s => s.tags)`. Tools don't carry tags, so we
            use the tool names themselves as the companion chips — unique
            within a server and informative at a glance. */}
        {toolCount > 0 && (
          <div className="flex flex-wrap gap-1 pt-1">
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 flex items-center gap-1">
              <Cpu className="h-2.5 w-2.5" aria-hidden="true" />
              {toolCount} tool{toolCount !== 1 ? 's' : ''}
            </Badge>
            {lv?.tools?.slice(0, 3).map((tool) => (
              <Badge
                key={tool.name}
                variant="outline"
                className="text-[10px] px-1.5 py-0 font-mono"
                title={tool.description || tool.name}
              >
                {tool.name}
              </Badge>
            ))}
          </div>
        )}
      </CardHeader>

      {server.description && (
        <CardContent className="pb-3 flex-1">
          <p className="line-clamp-2 text-sm text-foreground/80">
            {server.description}
          </p>
        </CardContent>
      )}

      {endpointUrl && (
        <CardContent className="pt-0 pb-3">
          <div className="flex items-center gap-2 min-w-0">
            {transportType && (
              <span className="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 text-[11px] font-sans font-semibold text-primary/80">
                {transportType}
              </span>
            )}
            <div className="flex items-center gap-1.5 rounded bg-muted/60 px-2 py-1.5 min-w-0 flex-1">
              <Link2 className="h-3 w-3 text-muted-foreground shrink-0" aria-hidden="true" />
              <span className="text-[11px] font-mono text-muted-foreground truncate" title={endpointUrl}>
                {endpointUrl}
              </span>
              <CopyButton value={endpointUrl} iconSize="h-3 w-3" label="Copy endpoint URL" />
            </div>
          </div>
        </CardContent>
      )}

      <CardFooter className="pt-3 border-t flex items-center justify-between gap-2 text-xs text-muted-foreground relative z-10">
        <div className="flex items-center gap-3 min-w-0">
          <FreshnessIndicator updatedAt={server.updated_at} />
          <span
            className="inline-flex items-center gap-1"
            title={`${(server.view_count ?? 0).toLocaleString()} views`}
            aria-label={`${(server.view_count ?? 0).toLocaleString()} views`}
          >
            <Eye className="h-3 w-3" aria-hidden="true" />
            {formatCount(server.view_count ?? 0)}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {server.license && <span>{server.license}</span>}
          <a
            href={`/api/v1/mcp/servers/${server.namespace}/${server.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
            aria-label="View JSON API response"
          >
            <Braces className="h-3.5 w-3.5" aria-hidden="true" />
            JSON
          </a>
          {server.repo_url && (
            <a
              href={server.repo_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 hover:text-foreground transition-colors"
              aria-label="View repository"
            >
              <GitFork className="h-3.5 w-3.5" aria-hidden="true" />
              Repo
            </a>
          )}
          {server.homepage_url && (
            <a
              href={server.homepage_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 hover:text-foreground transition-colors"
              aria-label="View documentation"
            >
              <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
              Docs
            </a>
          )}
        </div>
      </CardFooter>
    </Card>
  )
}
