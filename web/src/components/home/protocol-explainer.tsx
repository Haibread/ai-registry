/**
 * ProtocolExplainer — collapsible section explaining MCP and A2A protocols.
 * Shown below the hero section on the home page.
 */

import { useState } from 'react'
import { ChevronDown, ChevronRight, Plug, Bot } from 'lucide-react'

export function ProtocolExplainer() {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border rounded-lg bg-card">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-accent/50 transition-colors rounded-lg"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
      >
        <span>What are MCP and A2A?</span>
        {expanded ? (
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        ) : (
          <ChevronRight className="h-4 w-4 text-muted-foreground" />
        )}
      </button>

      {expanded && (
        <div className="px-4 pb-4 space-y-4 text-sm text-muted-foreground">
          <div className="flex gap-3">
            <Plug className="h-5 w-5 shrink-0 mt-0.5 text-primary" />
            <div>
              <p className="font-medium text-foreground">Model Context Protocol (MCP)</p>
              <p className="mt-1">
                MCP is an open standard for connecting AI models to external data sources
                and tools. MCP servers expose capabilities like database access, API
                integrations, and file operations that AI assistants can use via a
                standardized protocol.
              </p>
              <a
                href="https://modelcontextprotocol.io/"
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-primary hover:underline mt-1 inline-block"
              >
                Learn more at modelcontextprotocol.io &rarr;
              </a>
            </div>
          </div>

          <div className="flex gap-3">
            <Bot className="h-5 w-5 shrink-0 mt-0.5 text-primary" />
            <div>
              <p className="font-medium text-foreground">Agent-to-Agent Protocol (A2A)</p>
              <p className="mt-1">
                A2A is a protocol for AI agents to discover and communicate with each
                other. Each agent publishes an Agent Card describing its skills,
                authentication requirements, and endpoint URL, enabling automated
                agent-to-agent collaboration.
              </p>
              <a
                href="https://a2a-protocol.org/"
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-primary hover:underline mt-1 inline-block"
              >
                Learn more at a2a-protocol.org &rarr;
              </a>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
