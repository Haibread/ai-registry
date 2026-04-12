/**
 * FreshnessIndicator — shows a colored dot + relative time
 * indicating how recently an entry was updated.
 *
 * Green: < 3 months, Yellow: 3–12 months, Red: > 12 months.
 */

import { cn } from '@/lib/utils'

interface FreshnessIndicatorProps {
  updatedAt: string
  className?: string
}

function getRelativeTime(dateStr: string): { label: string; color: 'green' | 'yellow' | 'red' } {
  const now = new Date()
  const date = new Date(dateStr)
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays < 1) return { label: 'Today', color: 'green' }
  if (diffDays === 1) return { label: '1 day ago', color: 'green' }
  if (diffDays < 7) return { label: `${diffDays} days ago`, color: 'green' }
  if (diffDays < 30) {
    const weeks = Math.floor(diffDays / 7)
    return { label: `${weeks} week${weeks > 1 ? 's' : ''} ago`, color: 'green' }
  }
  if (diffDays < 90) {
    const months = Math.floor(diffDays / 30)
    return { label: `${months} month${months > 1 ? 's' : ''} ago`, color: 'green' }
  }
  if (diffDays < 365) {
    const months = Math.floor(diffDays / 30)
    return { label: `${months} months ago`, color: 'yellow' }
  }
  const years = Math.floor(diffDays / 365)
  return { label: years === 1 ? '1 year ago' : `${years} years ago`, color: 'red' }
}

const DOT_COLORS = {
  green: 'bg-green-500',
  yellow: 'bg-yellow-500',
  red: 'bg-red-500',
}

export function FreshnessIndicator({ updatedAt, className }: FreshnessIndicatorProps) {
  const { label, color } = getRelativeTime(updatedAt)

  return (
    <span className={cn('inline-flex items-center gap-1.5 text-xs text-muted-foreground', className)}>
      <span
        className={cn('h-2 w-2 rounded-full shrink-0', DOT_COLORS[color])}
        aria-hidden="true"
      />
      {label}
      {color === 'red' && (
        <span className="text-red-600 dark:text-red-400 font-medium">
          (stale)
        </span>
      )}
    </span>
  )
}
