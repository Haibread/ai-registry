/**
 * Badge variant styles + status/visibility mapping helpers.
 *
 * Split out of `badge.tsx` so that file only exports React components.
 * `react-refresh/only-export-components` wants one kind of export per module
 * so HMR can safely remount components without losing state.
 */

import { cva, type VariantProps } from "class-variance-authority"

export const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-hidden focus:ring-2 focus:ring-ring focus:ring-offset-2",
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
