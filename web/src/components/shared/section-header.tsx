/**
 * SectionHeader — small uppercase label used to introduce grouped metadata
 * sections on detail pages (Runtime, Release, Connection, etc).
 *
 * Deliberately minimal: keeps the Overview tab scannable without turning
 * each group into a heavy card with its own chrome.
 */

import { ReactNode } from 'react'

interface SectionHeaderProps {
  icon?: ReactNode
  title: string
}

export function SectionHeader({ icon, title }: SectionHeaderProps) {
  return (
    <div className="flex items-center gap-2 px-1">
      {icon && (
        <span className="flex items-center text-muted-foreground [&>svg]:h-3.5 [&>svg]:w-3.5">
          {icon}
        </span>
      )}
      <h2 className="text-[11px] uppercase tracking-wider font-semibold text-muted-foreground">
        {title}
      </h2>
    </div>
  )
}
