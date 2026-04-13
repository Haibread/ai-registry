import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"

export interface BreadcrumbSegment {
  label: string
  href?: string
}

interface BreadcrumbsProps {
  segments: BreadcrumbSegment[]
  className?: string
}

/**
 * A simple breadcrumb navigation bar.
 *
 * Segments with an `href` render as links; the last segment (current page)
 * renders as plain text.
 */
export function Breadcrumbs({ segments, className }: BreadcrumbsProps) {
  return (
    <nav
      aria-label="Breadcrumb"
      className={cn("flex items-center gap-1 text-sm text-muted-foreground", className)}
    >
      {segments.map((segment, i) => {
        const isLast = i === segments.length - 1
        return (
          <span key={i} className="flex items-center gap-1">
            {i > 0 && <ChevronRight className="h-3.5 w-3.5 shrink-0" />}
            {segment.href && !isLast ? (
              <Link
                to={segment.href}
                className="hover:text-foreground transition-colors"
              >
                {segment.label}
              </Link>
            ) : (
              <span
                className={cn(isLast && "text-foreground font-medium truncate max-w-[200px]")}
                title={isLast ? segment.label : undefined}
              >
                {segment.label}
              </span>
            )}
          </span>
        )
      })}
    </nav>
  )
}
