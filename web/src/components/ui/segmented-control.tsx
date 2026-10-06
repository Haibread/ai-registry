import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface SegmentedOption<T extends string> {
  value: T
  label: ReactNode
}

interface SegmentedControlProps<T extends string> {
  options: readonly SegmentedOption<T>[]
  value: T
  onChange: (value: T) => void
  /** Names the group for assistive tech. */
  label: string
  size?: 'sm' | 'default'
  className?: string
}

/** A row of mutually exclusive toggle buttons. The selected one is raised
 *  rather than filled with the primary color, so it never competes with the
 *  page's primary action. */
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  label,
  size = 'default',
  className,
}: SegmentedControlProps<T>) {
  return (
    <div
      role="group"
      aria-label={label}
      className={cn('inline-flex flex-wrap items-center gap-0.5 rounded-lg bg-muted p-1', className)}
    >
      {options.map((o) => {
        const selected = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(o.value)}
            className={cn(
              'inline-flex cursor-pointer items-center gap-1.5 rounded-md font-medium transition-colors',
              'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring',
              size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm',
              selected
                ? 'bg-background text-foreground shadow-sm dark:bg-accent'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}
