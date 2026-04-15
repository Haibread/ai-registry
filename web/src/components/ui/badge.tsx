import * as React from "react"
import { type VariantProps } from "class-variance-authority"
import { Circle, CheckCircle2, AlertTriangle, Globe, Lock, ShieldCheck } from "lucide-react"
import { cn } from "@/lib/utils"
import { badgeVariants, statusVariant, visibilityVariant } from "./badge-variants"

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

/** Status badge with icon prefix for colorblind-safe communication. */
function StatusBadge({ status, className }: { status: "draft" | "published" | "deprecated"; className?: string }) {
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
function VisibilityBadge({ visibility, className }: { visibility: "public" | "private"; className?: string }) {
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

/** Verified badge — shown when an entry has been verified by the registry. */
function VerifiedBadge({ className }: { className?: string }) {
  return (
    <Badge variant="success" className={className}>
      <ShieldCheck className="h-2.5 w-2.5" aria-hidden="true" />
      Verified
    </Badge>
  )
}

export { Badge, StatusBadge, VisibilityBadge, VerifiedBadge }
