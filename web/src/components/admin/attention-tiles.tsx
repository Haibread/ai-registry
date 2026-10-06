import { Link } from 'react-router-dom'
import { ArrowRight, type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface AttentionTile {
  key: string
  count: number
  label: string
  to: string
  icon: LucideIcon
  /** `attention`: work waiting on the caller; `danger`: something reported
   *  as wrong; `neutral`: unfinished work nobody is blocked on. */
  tone: 'attention' | 'danger' | 'neutral'
}

const ICON_TONE: Record<AttentionTile['tone'], string> = {
  attention: 'bg-highlight/15 text-highlight-foreground',
  danger: 'bg-destructive/10 text-destructive',
  neutral: 'bg-muted text-muted-foreground',
}

/** The "Needs you" strip at the top of an admin dashboard. Tiles with a zero
 *  count are dropped, and the strip disappears when nothing is left. */
export function AttentionTiles({ tiles }: { tiles: AttentionTile[] }) {
  const visible = tiles.filter((t) => t.count > 0)
  if (visible.length === 0) return null

  return (
    <section className="space-y-2" aria-label="Needs you">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Needs you</h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {visible.map(({ key, count, label, to, icon: Icon, tone }) => (
          <Link
            key={key}
            to={to}
            className="flex items-center gap-3 rounded-xl border bg-card p-4 transition-colors hover:border-primary/40"
          >
            <span className={cn('flex h-10 w-10 shrink-0 items-center justify-center rounded-lg', ICON_TONE[tone])}>
              <Icon className="h-5 w-5" aria-hidden="true" />
            </span>
            <span className="flex-1 text-sm text-muted-foreground">
              <span className="block text-2xl font-bold leading-tight text-foreground tabular-nums">{count}</span>
              {label}
            </span>
            <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </Link>
        ))}
      </div>
    </section>
  )
}
