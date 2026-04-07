import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary text-primary-foreground",
        secondary: "border-transparent bg-secondary text-secondary-foreground",
        destructive: "border-transparent bg-destructive text-destructive-foreground",
        outline: "text-foreground",
        success: "border-transparent bg-green-100 text-green-800",
        warning: "border-transparent bg-yellow-100 text-yellow-800",
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

export { Badge, badgeVariants }
