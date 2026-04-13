/**
 * CapabilitiesSection — renders MCP server capabilities as labeled badges
 * with expandable tool/resource/prompt lists.
 *
 * The `capabilities` field is a free-form JSONB object whose keys are
 * capability names (tools, resources, prompts, logging, etc.) and whose
 * values describe the capability. For tools/resources/prompts, the value
 * may have a `listChanged` boolean or be an object with details.
 */

import { useState } from 'react'
import { ChevronDown, ChevronRight, Wrench, FileText, MessageSquare, Activity } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

interface CapabilitiesSectionProps {
  capabilities: Record<string, unknown>
  /** Hide the built-in section heading — useful when embedding inside a
   *  larger card that already has its own header. */
  hideTitle?: boolean
}

const CAPABILITY_META: Record<string, { label: string; icon: React.ComponentType<{ className?: string }> }> = {
  tools: { label: 'Tools', icon: Wrench },
  resources: { label: 'Resources', icon: FileText },
  prompts: { label: 'Prompts', icon: MessageSquare },
  logging: { label: 'Logging', icon: Activity },
}

export function CapabilitiesSection({ capabilities, hideTitle }: CapabilitiesSectionProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  const entries = Object.entries(capabilities).filter(
    ([, v]) => v !== null && v !== undefined && v !== false,
  )

  if (entries.length === 0) return null

  return (
    <div className="space-y-3">
      {!hideTitle && (
        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
          Capabilities
        </h3>
      )}
      <div className="flex flex-wrap gap-2">
        {entries.map(([key, value]) => {
          const meta = CAPABILITY_META[key]
          const Icon = meta?.icon
          const label = meta?.label ?? key
          const isExpandable = typeof value === 'object' && value !== null

          return (
            <div key={key}>
              {isExpandable ? (
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 gap-1.5 text-xs"
                  onClick={() =>
                    setExpanded((prev) => ({ ...prev, [key]: !prev[key] }))
                  }
                >
                  {Icon && <Icon className="h-3.5 w-3.5" />}
                  {label}
                  {expanded[key] ? (
                    <ChevronDown className="h-3 w-3" />
                  ) : (
                    <ChevronRight className="h-3 w-3" />
                  )}
                </Button>
              ) : (
                <Badge variant="secondary" className="gap-1.5 text-xs">
                  {Icon && <Icon className="h-3.5 w-3.5" />}
                  {label}
                </Badge>
              )}
              {expanded[key] && isExpandable && (
                <div className="mt-1.5 rounded-md bg-muted/50 px-3 py-2 text-xs font-mono overflow-x-auto max-h-48 overflow-y-auto">
                  <pre className="whitespace-pre-wrap break-words">
                    {JSON.stringify(value, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
