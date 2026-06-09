import { AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ErrorStateProps {
  message?: string
  className?: string
}

/**
 * A consistent inline error surface (`role="alert"`) for failed queries and
 * mutations. Pair with `problemMessage()` to render the server's RFC 7807
 * detail when there is one.
 */
export function ErrorState({ message, className }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        'flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive',
        className,
      )}
    >
      <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
      <span>{message ?? 'Something went wrong. Please try again.'}</span>
    </div>
  )
}
