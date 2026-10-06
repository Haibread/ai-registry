import { Package, PencilLine } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { InstallCommand } from '@/components/ui/install-command'
import { EmptyState } from '@/components/ui/empty-state'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { MarkdownRenderer } from '@/components/ui/markdown-renderer'
import { MCPConfigGenerator } from '@/components/mcp/config-generator'
import { MCPCodeSnippets } from '@/components/mcp/code-snippets'
import { getFieldExplanation } from '@/lib/field-explanations'
import { renderUsageMarkdown, usageValues, type UsageVersion } from '@/lib/usage-markdown'
import { ecosystemLabel, getInstallCommand, isRemoteTransport } from '@/lib/utils'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

interface UsageTabProps {
  server: MCPServer
  /** The version the page shows, which is not always `server.latest_version`. */
  version: UsageVersion | undefined
  onCopy?: () => void
}

export function UsageTab({ server, version: lv, onCopy }: UsageTabProps) {
  const hasGenerated = (lv?.packages?.length ?? 0) > 0 || (lv?.remotes?.length ?? 0) > 0
  const custom = server.usage_markdown ?? ''

  if (custom) {
    return (
      <div className="space-y-4">
        <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <PencilLine className="h-3.5 w-3.5" aria-hidden="true" />
          Written by <span className="font-medium text-foreground">{server.namespace}</span>
        </p>
        <MarkdownRenderer content={renderUsageMarkdown(custom, usageValues(server, lv))} />
        {hasGenerated && (
          <details className="group rounded-lg border px-4 py-3">
            <summary className="cursor-pointer text-sm font-medium">
              Generated configuration{' '}
              <span className="font-normal text-muted-foreground">— host config &amp; SDK snippets</span>
            </summary>
            <div className="mt-4 space-y-6">
              <GeneratedUsage server={server} version={lv} onCopy={onCopy} />
            </div>
          </details>
        )}
      </div>
    )
  }

  if (!hasGenerated) {
    return (
      <EmptyState
        icon={<Package className="h-8 w-8 text-muted-foreground" />}
        title="No packages available"
        description="This server has no published packages or remote endpoints yet."
      />
    )
  }
  return <GeneratedUsage server={server} version={lv} onCopy={onCopy} />
}

function GeneratedUsage({ server, version: lv, onCopy }: UsageTabProps) {
  return (
    <>
      <div className="space-y-3">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Package className="h-4 w-4" aria-hidden="true" />
          Connect
        </h2>
        <div className="space-y-4">
          {(lv?.remotes ?? []).map((remote, i) => (
            <div key={`remote-${i}`} className="space-y-1.5">
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="secondary" className="text-xs">remote</Badge>
                <Badge variant="outline" className="text-xs">{remote.type}</Badge>
                {getFieldExplanation(remote.type) && (
                  <TooltipInfo content={getFieldExplanation(remote.type)!} />
                )}
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Endpoint URL</p>
                <InstallCommand command={remote.url} onCopy={onCopy} />
              </div>
            </div>
          ))}
          {(lv?.packages ?? []).map((pkg, i) => {
            const remote = isRemoteTransport(pkg.transport.type)
            return (
              <div key={i} className="space-y-1.5">
                <div className="flex items-center gap-2 flex-wrap">
                  <Badge variant="secondary" className="text-xs">{ecosystemLabel(pkg.registryType)}</Badge>
                  <span className="text-xs text-muted-foreground font-mono truncate">{pkg.identifier}@{pkg.version}</span>
                  <Badge variant="outline" className="text-xs">{pkg.transport.type}</Badge>
                  {getFieldExplanation(pkg.transport.type) && (
                    <TooltipInfo content={getFieldExplanation(pkg.transport.type)!} />
                  )}
                </div>
                {remote && pkg.transport.url ? (
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">Endpoint URL</p>
                    <InstallCommand command={pkg.transport.url} onCopy={onCopy} />
                  </div>
                ) : (
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">Run command</p>
                    <InstallCommand command={getInstallCommand(pkg)} onCopy={onCopy} />
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
      <Separator />
      <MCPConfigGenerator
        serverName={server.slug}
        packages={lv?.packages ?? []}
        remotes={lv?.remotes ?? []}
      />
      <Separator />
      <MCPCodeSnippets
        serverName={server.slug}
        packages={lv?.packages ?? []}
        remotes={lv?.remotes ?? []}
        tools={lv?.tools}
      />
    </>
  )
}
