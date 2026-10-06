/**
 * MCPConfigGenerator — generates host-specific config snippets for an MCP server.
 *
 * Shown in the Usage tab of the MCP detail page. User selects a host
 * (Claude Desktop, Cursor, etc.) and a package, and the component generates
 * the exact JSON config block they need.
 */

import { useState } from 'react'
import { CodeBlock } from '@/components/ui/code-block'
import {
  MCP_HOSTS,
  connectionSources,
  type ConnectionPackage,
  type ConnectionRemote,
} from '@/lib/mcp-host-configs'

interface MCPConfigGeneratorProps {
  serverName: string
  packages: ConnectionPackage[]
  remotes?: ConnectionRemote[]
}

export function MCPConfigGenerator({ serverName, packages, remotes = [] }: MCPConfigGeneratorProps) {
  const [hostIndex, setHostIndex] = useState(0)
  const [sourceIndex, setSourceIndex] = useState(0)

  const sources = connectionSources(serverName, packages, remotes)
  if (sources.length === 0) return null

  const host = MCP_HOSTS[hostIndex]
  const params = sources[Math.min(sourceIndex, sources.length - 1)].params
  const snippet = host.generate(params)

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">Host Configuration</h3>
      <p className="text-xs text-muted-foreground">
        Generate a ready-to-paste config snippet for your MCP host.
      </p>

      <div className="flex flex-wrap gap-2">
        {/* Host selector */}
        <select
          value={hostIndex}
          onChange={(e) => setHostIndex(Number(e.target.value))}
          className="h-8 rounded-md border border-input bg-background px-2 text-sm"
          aria-label="Select MCP host"
        >
          {MCP_HOSTS.map((h, i) => (
            <option key={h.name} value={i}>
              {h.name}
            </option>
          ))}
        </select>

        {/* Connection selector (only shown if multiple packages/remotes) */}
        {sources.length > 1 && (
          <select
            value={sourceIndex}
            onChange={(e) => setSourceIndex(Number(e.target.value))}
            className="h-8 rounded-md border border-input bg-background px-2 text-sm"
            aria-label="Select connection"
          >
            {sources.map((s, i) => (
              <option key={i} value={i}>
                {s.label}
              </option>
            ))}
          </select>
        )}
      </div>

      {/* Config path hint */}
      <p className="text-xs text-muted-foreground">
        Add to <code className="font-mono text-xs bg-muted px-1 rounded">{host.configPath}</code>
      </p>

      {/* Generated snippet */}
      <CodeBlock value={snippet} copyLabel="Copy config" />
    </div>
  )
}
