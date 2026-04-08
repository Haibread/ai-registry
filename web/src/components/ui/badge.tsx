import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Circle, CheckCircle2, AlertTriangle, Globe, Lock } from "lucide-react"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary text-primary-foreground",
        secondary: "border-transparent bg-secondary text-secondary-foreground",
        destructive: "border-transparent bg-destructive text-destructive-foreground",
        outline: "text-foreground",
        success: "border-transparent bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
        warning: "border-transparent bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
        muted: "border-transparent bg-muted text-muted-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

/** Returns the badge variant for a status value. */
export function statusVariant(
  status: "draft" | "published" | "deprecated"
): VariantProps<typeof badgeVariants>["variant"] {
  switch (status) {
    case "published":
      return "success"
    case "deprecated":
      return "destructive"
    case "draft":
    default:
      return "muted"
  }
}

/** Returns the badge variant for a visibility value. */
export function visibilityVariant(
  visibility: "public" | "private"
): VariantProps<typeof badgeVariants>["variant"] {
  return visibility === "public" ? "default" : "secondary"
}

/** Status badge with icon prefix for colorblind-safe communication. */
export function StatusBadge({ status, className }: { status: "draft" | "published" | "deprecated"; className?: string }) {
  const icons = {
    draft: <Circle className="h-2.5 w-2.5" aria-hidden="true" />,
    published: <CheckCircle2 className="h-2.5 w-2.5" aria-hidden="true" />,
    deprecated: <AlertTriangle className="h-2.5 w-2.5" aria-hidden="true" />,
  }
  return (
    <Badge variant={statusVariant(status)} className={className}>
      {icons[status]}
      {status}
    </Badge>
  )
}

/** Visibility badge with icon prefix. */
export function VisibilityBadge({ visibility, className }: { visibility: "public" | "private"; className?: string }) {
  return (
    <Badge variant={visibilityVariant(visibility)} className={className}>
      {visibility === "public"
        ? <Globe className="h-2.5 w-2.5" aria-hidden="true" />
        : <Lock className="h-2.5 w-2.5" aria-hidden="true" />
      }
      {visibility}
    </Badge>
  )
}

export { Badge, badgeVariants }
