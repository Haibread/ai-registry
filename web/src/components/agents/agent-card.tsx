import Link from "next/link"
import { ExternalLink } from "lucide-react"
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge, statusVariant } from "@/components/ui/badge"
import { formatDate } from "@/lib/utils"
import type { components } from "@/lib/schema"

type Agent = components["schemas"]["Agent"]

interface AgentCardProps {
  agent: Agent
}

export function AgentCard({ agent }: AgentCardProps) {
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
          <Badge variant={statusVariant(agent.status)} className="shrink-0 text-[11px]">
            {agent.status}
          </Badge>
        </div>
        <div className="text-xs text-muted-foreground font-mono">
          {agent.namespace}/{agent.slug}
        </div>
      </CardHeader>

      {agent.description && (
        <CardContent className="pb-3 flex-1">
          <CardDescription className="line-clamp-2 text-sm">
            {agent.description}
          </CardDescription>
        </CardContent>
      )}

      <CardFooter className="pt-3 border-t flex items-center justify-between text-xs text-muted-foreground">
        <span>{formatDate(agent.created_at)}</span>
        <a
          href={`/agents/${agent.namespace}/${agent.slug}/.well-known/agent-card.json`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 hover:text-foreground transition-colors"
        >
          <ExternalLink className="h-3.5 w-3.5" />
          A2A card
        </a>
      </CardFooter>
    </Card>
  )
}
