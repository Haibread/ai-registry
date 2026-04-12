import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  icon: ReactNode
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

/**
 * A centered empty state placeholder for when a list has no results
 * or a section has no data.
 */
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center gap-3 py-16 text-center",
        className,
      )}
    >
      <div className="text-muted-foreground/40">{icon}</div>
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      {description && (
        <p className="text-xs text-muted-foreground/70 max-w-xs">{description}</p>
      )}
      {action && <div className="mt-1">{action}</div>}
    </div>
  )
}
