/**
 * MCPConfigGenerator — generates host-specific config snippets for an MCP server.
 *
 * Shown in the Installation tab of the MCP detail page. User selects a host
 * (Claude Desktop, Cursor, etc.) and a package, and the component generates
 * the exact JSON config block they need.
 */

import { useState } from 'react'
import { CopyButton } from '@/components/ui/copy-button'
import { MCP_HOSTS, packageToConfigParams } from '@/lib/mcp-host-configs'

interface Package {
  registryType: string
  identifier: string
  version: string
  transport: { type: string; url?: string }
}

interface MCPConfigGeneratorProps {
  serverName: string
  packages: Package[]
}

export function MCPConfigGenerator({ serverName, packages }: MCPConfigGeneratorProps) {
  const [hostIndex, setHostIndex] = useState(0)
  const [pkgIndex, setPkgIndex] = useState(0)

  if (packages.length === 0) return null

  const host = MCP_HOSTS[hostIndex]
  const pkg = packages[pkgIndex]
  const params = packageToConfigParams(serverName, pkg)
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

        {/* Package selector (only shown if multiple packages) */}
        {packages.length > 1 && (
          <select
            value={pkgIndex}
            onChange={(e) => setPkgIndex(Number(e.target.value))}
            className="h-8 rounded-md border border-input bg-background px-2 text-sm"
            aria-label="Select package"
          >
            {packages.map((p, i) => (
              <option key={i} value={i}>
                {p.identifier} ({p.transport.type})
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
      <div className="relative rounded-md bg-muted overflow-hidden">
        <div className="absolute top-2 right-2 z-10">
          <CopyButton value={snippet} label="Copy config" />
        </div>
        <pre className="p-3 pr-12 text-xs font-mono overflow-x-auto whitespace-pre">
          {snippet}
        </pre>
      </div>
    </div>
  )
}
