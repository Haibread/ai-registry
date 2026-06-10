/**
 * NativeSelect — the shared native <select> primitive (UI/UX review P3).
 *
 * Deliberate split in the select vocabulary: rich create/edit forms use the
 * Radix Select (searchable trigger, custom item rendering); compact inline
 * controls — filter bars, the grants editor — use this styled native select,
 * which is denser and keeps platform keyboard/touch behavior. Use one of the
 * two; never a hand-styled bare <select>.
 */

import * as React from 'react'
import { cn } from '@/lib/utils'

const NativeSelect = React.forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, ...props }, ref) => (
    <select
      ref={ref}
      className={cn(
        'h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground shadow-xs',
        'transition-colors focus-visible:outline-hidden focus-visible:ring-1 focus-visible:ring-ring',
        className,
      )}
      {...props}
    />
  ),
)
NativeSelect.displayName = 'NativeSelect'

export { NativeSelect }
