import { AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface ErrorStateProps {
  message?: string
  className?: string
  /** Renders a "Try again" button wired to the failed query's refetch. */
  onRetry?: () => void
}

/**
 * A consistent inline error surface (`role="alert"`) for failed queries and
 * mutations. Pair with `problemMessage()` to render the server's RFC 7807
 * detail when there is one.
 *
 * Error-display rule (one dialect per situation, not per page):
 * - Field validation errors → inline under the field (`aria-invalid` +
 *   `role="alert"` text, e.g. SlugField).
 * - Form/query errors → this component (or the form's alert region) next to
 *   the thing that failed, with a retry when refetching can help.
 * - Background-mutation errors (the user has moved on) → `toast.error`.
 */
export function ErrorState({ message, className, onRetry }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        'flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive',
        className,
      )}
    >
      <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
      <span className="flex-1">{message ?? 'Something went wrong. Please try again.'}</span>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry} className="shrink-0">
          Try again
        </Button>
      )}
    </div>
  )
}
