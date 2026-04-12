/**
 * CompatibilityInfo — displays protocol version and transport types.
 *
 * Simple initial version; can be extended with tested_with data later.
 */

import { Badge } from '@/components/ui/badge'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { getFieldExplanation } from '@/lib/field-explanations'

interface CompatibilityInfoProps {
  protocolVersion?: string
  transportTypes?: string[]
}

export function CompatibilityInfo({ protocolVersion, transportTypes }: CompatibilityInfoProps) {
  if (!protocolVersion && (!transportTypes || transportTypes.length === 0)) return null

  return (
    <div className="space-y-2">
      <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
        Compatibility
      </h4>
      <div className="flex flex-wrap gap-2 items-center">
        {protocolVersion && (
          <span className="flex items-center gap-1">
            <Badge variant="outline" className="text-xs font-mono">
              Protocol {protocolVersion}
            </Badge>
            {getFieldExplanation('protocol_version') && (
              <TooltipInfo content={getFieldExplanation('protocol_version')!} />
            )}
          </span>
        )}
        {transportTypes &&
          transportTypes.map((t) => (
            <span key={t} className="flex items-center gap-1">
              <Badge variant="secondary" className="text-xs">
                {t}
              </Badge>
              {getFieldExplanation(t) && (
                <TooltipInfo content={getFieldExplanation(t)!} />
              )}
            </span>
          ))}
      </div>
    </div>
  )
}
