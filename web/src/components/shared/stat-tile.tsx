/**
 * StatTile — one labelled fact inside a metadata card on detail pages.
 *
 * Deliberately borderless: tiles are meant to be rendered inside a single
 * wrapping card, separated only by dividers so the whole metadata section
 * reads as one cohesive panel instead of a wall of nested boxes.
 *
 * The optional `icon` slot lets each tile carry a small lucide glyph next
 * to its label to establish visual rhythm across the row.
 */

import { ReactNode } from 'react'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { cn } from '@/lib/utils'

interface StatTileProps {
  label: string
  tooltip?: string
  icon?: ReactNode
  children: ReactNode
  className?: string
}

export function StatTile({ label, tooltip, icon, children, className }: StatTileProps) {
  return (
    <div className={cn('space-y-1.5 min-w-0', className)}>
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-semibold text-muted-foreground/80">
        {icon && (
          <span className="flex items-center text-muted-foreground/60 [&>svg]:h-3 [&>svg]:w-3">
            {icon}
          </span>
        )}
        <span>{label}</span>
        {tooltip && <TooltipInfo content={tooltip} />}
      </div>
      <div className="text-[15px] leading-snug text-foreground min-w-0">{children}</div>
    </div>
  )
}
