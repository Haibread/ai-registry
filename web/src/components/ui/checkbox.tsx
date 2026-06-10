/**
 * Checkbox — the shared checkbox primitive (UI/UX review P3).
 *
 * A styled native <input type="checkbox"> rather than a Radix widget: native
 * checkboxes keep form semantics, label association, and keyboard behavior
 * for free. This wrapper adds the design system's focus ring (the bare
 * inputs previously had no focus styling at all) and supports the
 * indeterminate state for "some rows selected" table headers.
 */

import * as React from 'react'
import { cn } from '@/lib/utils'

interface CheckboxProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** Renders the indeterminate ("some selected") state. DOM-only property,
   *  so it's applied via a ref rather than an attribute. */
  indeterminate?: boolean
}

const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, indeterminate = false, ...props }, forwardedRef) => {
    const innerRef = React.useRef<HTMLInputElement>(null)
    React.useImperativeHandle(forwardedRef, () => innerRef.current as HTMLInputElement)

    React.useEffect(() => {
      if (innerRef.current) innerRef.current.indeterminate = indeterminate
    }, [indeterminate])

    return (
      <input
        ref={innerRef}
        type="checkbox"
        className={cn(
          'h-4 w-4 shrink-0 rounded border border-input accent-primary cursor-pointer',
          'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ring-offset-background',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className,
        )}
        {...props}
      />
    )
  },
)
Checkbox.displayName = 'Checkbox'

export { Checkbox }
