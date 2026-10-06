import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { ResourceIcon, type ResourceType } from '@/components/ui/resource-icon'

interface Tile {
  type: ResourceType
  title: string
  description: string
  to: string
  noun: string
  learnMoreUrl: string
  learnMoreLabel: string
}

const TILES: Tile[] = [
  {
    type: 'mcp-server',
    title: 'MCP servers',
    description:
      'The Model Context Protocol gives AI models tools and data: databases, APIs, files, through one standard interface.',
    to: '/mcp',
    noun: 'server',
    learnMoreUrl: 'https://modelcontextprotocol.io/',
    learnMoreLabel: 'What is MCP?',
  },
  {
    type: 'agent',
    title: 'A2A agents',
    description:
      'Agents that discover and call each other over the Agent-to-Agent protocol, each described by an Agent Card listing its skills, auth and endpoint.',
    to: '/agents',
    noun: 'agent',
    learnMoreUrl: 'https://a2a-protocol.org/',
    learnMoreLabel: 'What is A2A?',
  },
]

interface ProtocolTilesProps {
  mcpCount?: number | undefined
  agentCount?: number | undefined
}

/** Home entry points: what each protocol is, and a way into its catalog. */
export function ProtocolTiles({ mcpCount, agentCount }: ProtocolTilesProps) {
  const counts: Record<string, number | undefined> = { '/mcp': mcpCount, '/agents': agentCount }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      {TILES.map((tile) => {
        const count = counts[tile.to]
        const browse = count == null ? `Browse ${tile.noun}s` : `Browse ${count} ${tile.noun}${count === 1 ? '' : 's'}`
        return (
          <div
            key={tile.to}
            className="relative flex gap-4 rounded-xl border bg-card p-5 transition-colors hover:border-primary/40 hover:shadow-sm"
          >
            <div
              className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-secondary text-secondary-foreground"
              aria-hidden="true"
            >
              <ResourceIcon type={tile.type} className="h-5 w-5" />
            </div>
            <div className="min-w-0 space-y-1.5">
              <h2 className="text-lg font-semibold">{tile.title}</h2>
              <p className="text-sm text-muted-foreground">{tile.description}</p>
              <div className="flex flex-wrap items-center gap-x-4 gap-y-1 pt-1 text-sm">
                <Link
                  to={tile.to}
                  className="inline-flex items-center gap-1.5 font-semibold text-primary after:absolute after:inset-0 after:content-['']"
                >
                  {browse} <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </Link>
                <a
                  href={tile.learnMoreUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="relative z-10 text-muted-foreground hover:text-foreground hover:underline"
                >
                  {tile.learnMoreLabel}
                </a>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
