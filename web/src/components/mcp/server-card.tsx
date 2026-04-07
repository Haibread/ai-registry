import Link from "next/link"
import { ExternalLink, GitFork } from "lucide-react"
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge, statusVariant } from "@/components/ui/badge"
import { formatDate } from "@/lib/utils"
import type { components } from "@/lib/schema"

type MCPServer = components["schemas"]["MCPServer"]

interface ServerCardProps {
  server: MCPServer
}

export function ServerCard({ server }: ServerCardProps) {
  return (
    <Card className="flex flex-col hover:shadow-md transition-shadow">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-base leading-snug">
            <Link
              href={`/mcp/${server.namespace}/${server.slug}`}
              className="hover:text-primary transition-colors"
            >
              {server.name}
            </Link>
          </CardTitle>
          <Badge variant={statusVariant(server.status)} className="shrink-0 text-[11px]">
            {server.status}
          </Badge>
        </div>
        <div className="text-xs text-muted-foreground font-mono">
          {server.namespace}/{server.slug}
        </div>
      </CardHeader>

      {server.description && (
        <CardContent className="pb-3 flex-1">
          <CardDescription className="line-clamp-2 text-sm">
            {server.description}
          </CardDescription>
        </CardContent>
      )}

      <CardFooter className="pt-3 border-t flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{formatDate(server.created_at)}</span>
        <div className="flex items-center gap-2">
          {server.license && <span>{server.license}</span>}
          {server.repo_url && (
            <a
              href={server.repo_url}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-foreground transition-colors"
              aria-label="Repository"
            >
              <GitFork className="h-3.5 w-3.5" />
            </a>
          )}
          {server.homepage_url && (
            <a
              href={server.homepage_url}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-foreground transition-colors"
              aria-label="Homepage"
            >
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
        </div>
      </CardFooter>
    </Card>
  )
}
