import Link from "next/link"
import { ExternalLink, GitFork, Braces } from "lucide-react"
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge, statusVariant } from "@/components/ui/badge"
import { formatDate, ecosystemLabel } from "@/lib/utils"
import type { components } from "@/lib/schema"

type MCPServer = components["schemas"]["MCPServer"]

interface ServerCardProps {
  server: MCPServer
}

export function ServerCard({ server }: ServerCardProps) {
  const lv = server.latest_version

  // Collect unique ecosystems from packages
  const ecosystems = lv?.packages
    ? [...new Set(lv.packages.map((p) => ecosystemLabel(p.registryType)))]
    : []

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
          <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
            {lv && (
              <Badge variant="outline" className="text-[11px] font-mono">
                v{lv.version}
              </Badge>
            )}
            <Badge variant={statusVariant(server.status)} className="text-[11px]">
              {server.status}
            </Badge>
          </div>
        </div>
        <div className="text-xs text-muted-foreground font-mono">
          {server.namespace}/{server.slug}
        </div>

        {/* Runtime + ecosystem chips */}
        {lv && (
          <div className="flex flex-wrap gap-1 pt-1">
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
              {lv.runtime}
            </Badge>
            {ecosystems.map((eco) => (
              <Badge key={eco} variant="secondary" className="text-[10px] px-1.5 py-0">
                {eco}
              </Badge>
            ))}
          </div>
        )}
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
          <a
            href={`/api/v1/mcp/servers/${server.namespace}/${server.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-foreground transition-colors"
            aria-label="Raw JSON"
          >
            <Braces className="h-3.5 w-3.5" />
          </a>
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
