import { Link } from 'react-router-dom'
import { ExternalLink, Eye, Braces, Cpu, Link2, Bot } from 'lucide-react'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge, StatusBadge, VerifiedBadge } from '@/components/ui/badge'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { formatCount } from '@/lib/utils'
import type { components } from '@/lib/schema'

type Agent = components['schemas']['Agent']

interface AgentCardProps {
  agent: Agent
}

export function AgentCard({ agent }: AgentCardProps) {
  const lv = agent.latest_version
  const to = `/agents/${agent.namespace}/${agent.slug}`

  return (
    <Card className="flex flex-col hover:shadow-md transition-shadow group relative">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-base leading-snug flex items-center gap-2 min-w-0">
            <div className="rounded-md bg-primary/10 p-1.5 shrink-0" aria-hidden="true">
              <Bot className="h-4 w-4 text-primary" />
            </div>
            <Link
              to={to}
              className="truncate hover:text-primary transition-colors after:absolute after:inset-0 after:content-['']"
            >
              {agent.name}
            </Link>
          </CardTitle>
          <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end relative z-10">
            {lv && (
              <Badge variant="outline" className="text-[11px] font-mono">
                v{lv.version}
              </Badge>
            )}
            {agent.verified && <VerifiedBadge className="text-[10px]" />}
            <StatusBadge status={agent.status} className="text-[11px]" />
          </div>
        </div>
        <div className="text-xs text-muted-foreground font-mono relative z-10">
          <Link
            to={`/agents/${agent.namespace}`}
            className="hover:text-foreground transition-colors"
          >
            {agent.namespace}
          </Link>
          /{agent.slug}
        </div>

        {/* Skills count + top tags */}
        {lv?.skills && lv.skills.length > 0 && (
          <div className="flex flex-wrap gap-1 pt-1">
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 flex items-center gap-1">
              <Cpu className="h-2.5 w-2.5" aria-hidden="true" />
              {lv.skills.length} skill{lv.skills.length !== 1 ? 's' : ''}
            </Badge>
            {(() => {
              // Collect unique tags across all skills, show up to 3.
              const tags = [...new Set(lv.skills.flatMap(s => s.tags ?? []))].slice(0, 3)
              return tags.map(tag => (
                <Badge key={tag} variant="outline" className="text-[10px] px-1.5 py-0">
                  {tag}
                </Badge>
              ))
            })()}
          </div>
        )}
      </CardHeader>

      {agent.description && (
        <CardContent className="pb-3 flex-1">
          <p className="line-clamp-2 text-sm text-foreground/80">
            {agent.description}
          </p>
        </CardContent>
      )}

      {lv?.endpoint_url && (
        <CardContent className="pt-0 pb-3">
          <div className="flex items-center gap-1.5 rounded bg-muted/60 px-2 py-1.5 min-w-0">
            <Link2 className="h-3 w-3 text-muted-foreground shrink-0" aria-hidden="true" />
            <span className="text-[11px] font-mono text-muted-foreground truncate" title={lv.endpoint_url}>
              {lv.endpoint_url}
            </span>
          </div>
        </CardContent>
      )}

      <CardFooter className="pt-3 border-t flex items-center justify-between text-xs text-muted-foreground relative z-10">
        <div className="flex items-center gap-3 min-w-0">
          <FreshnessIndicator updatedAt={agent.updated_at} />
          <span
            className="inline-flex items-center gap-1"
            title={`${(agent.view_count ?? 0).toLocaleString()} views`}
            aria-label={`${(agent.view_count ?? 0).toLocaleString()} views`}
          >
            <Eye className="h-3 w-3" aria-hidden="true" />
            {formatCount(agent.view_count ?? 0)}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <a
            href={`/api/v1/agents/${agent.namespace}/${agent.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
            aria-label="View JSON API response"
          >
            <Braces className="h-3.5 w-3.5" aria-hidden="true" />
            JSON
          </a>
          <a
            href={`/agents/${agent.namespace}/${agent.slug}/.well-known/agent-card.json`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
            aria-label="View A2A agent card"
          >
            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
            A2A card
          </a>
        </div>
      </CardFooter>
    </Card>
  )
}
