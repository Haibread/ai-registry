import Link from "next/link"
import { ExternalLink, Braces, Cpu } from "lucide-react"
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge, statusVariant } from "@/components/ui/badge"
import { formatDate } from "@/lib/utils"
import type { components } from "@/lib/schema"

type Agent = components["schemas"]["Agent"]

interface AgentCardProps {
  agent: Agent
}

export function AgentCard({ agent }: AgentCardProps) {
  const lv = agent.latest_version

  return (
    <Card className="flex flex-col hover:shadow-md transition-shadow">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-base leading-snug">
            <Link
              href={`/agents/${agent.namespace}/${agent.slug}`}
              className="hover:text-primary transition-colors"
            >
              {agent.name}
            </Link>
          </CardTitle>
          <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
            {lv && (
              <Badge variant="outline" className="text-[11px] font-mono">
                v{lv.version}
              </Badge>
            )}
            <Badge variant={statusVariant(agent.status)} className="text-[11px]">
              {agent.status}
            </Badge>
          </div>
        </div>
        <div className="text-xs text-muted-foreground font-mono">
          {agent.namespace}/{agent.slug}
        </div>

        {/* Skills count + I/O modes */}
        {lv && (
          <div className="flex flex-wrap gap-1 pt-1">
            {lv.skills && lv.skills.length > 0 && (
              <Badge variant="secondary" className="text-[10px] px-1.5 py-0 flex items-center gap-1">
                <Cpu className="h-2.5 w-2.5" />
                {lv.skills.length} skill{lv.skills.length !== 1 ? "s" : ""}
              </Badge>
            )}
            {lv.default_input_modes?.map((m) => (
              <Badge key={`in-${m}`} variant="secondary" className="text-[10px] px-1.5 py-0">
                ↓ {m}
              </Badge>
            ))}
            {lv.default_output_modes?.map((m) => (
              <Badge key={`out-${m}`} variant="secondary" className="text-[10px] px-1.5 py-0">
                ↑ {m}
              </Badge>
            ))}
          </div>
        )}
      </CardHeader>

      {agent.description && (
        <CardContent className="pb-3 flex-1">
          <CardDescription className="line-clamp-2 text-sm">
            {agent.description}
          </CardDescription>
        </CardContent>
      )}

      {lv?.endpoint_url && (
        <CardContent className="pt-0 pb-2">
          <p className="text-xs text-muted-foreground font-mono truncate" title={lv.endpoint_url}>
            {lv.endpoint_url}
          </p>
        </CardContent>
      )}

      <CardFooter className="pt-3 border-t flex items-center justify-between text-xs text-muted-foreground">
        <span>{formatDate(agent.created_at)}</span>
        <div className="flex items-center gap-2">
          <a
            href={`/api/v1/agents/${agent.namespace}/${agent.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
          >
            <Braces className="h-3.5 w-3.5" />
            JSON
          </a>
          <a
            href={`/agents/${agent.namespace}/${agent.slug}/.well-known/agent-card.json`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            A2A card
          </a>
        </div>
      </CardFooter>
    </Card>
  )
}
