import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { ResourceIcon } from '@/components/ui/resource-icon'

interface ProtocolTileProps {
  mcpCount?: number | undefined
}

/** Home entry point: what MCP is, and a way into the catalog. */
export function ProtocolTile({ mcpCount }: ProtocolTileProps) {
  const browse = mcpCount == null ? 'Browse servers' : `Browse ${mcpCount} server${mcpCount === 1 ? '' : 's'}`

  return (
    <div className="relative flex gap-4 rounded-xl border bg-card p-5 transition-colors hover:border-primary/40 hover:shadow-sm">
      <div
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-secondary text-secondary-foreground"
        aria-hidden="true"
      >
        <ResourceIcon type="mcp-server" className="h-5 w-5" />
      </div>
      <div className="min-w-0 space-y-1.5">
        <h2 className="text-lg font-semibold">MCP servers</h2>
        <p className="text-sm text-muted-foreground">
          The Model Context Protocol gives AI models tools and data: databases, APIs, files, through one standard interface.
        </p>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 pt-1 text-sm">
          <Link
            to="/mcp"
            className="inline-flex items-center gap-1.5 font-semibold text-primary after:absolute after:inset-0 after:content-['']"
          >
            {browse} <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
          <a
            href="https://modelcontextprotocol.io/"
            target="_blank"
            rel="noopener noreferrer"
            className="relative z-10 text-muted-foreground hover:text-foreground hover:underline"
          >
            What is MCP?
          </a>
        </div>
      </div>
    </div>
  )
}
