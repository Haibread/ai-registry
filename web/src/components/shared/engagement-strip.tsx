/**
 * EngagementStrip — compact inline row of low-priority engagement metrics
 * (views, installs) and provenance timestamps (created, updated).
 *
 * Rendered borderless and muted-foreground-sized so it reads as a footnote
 * on detail pages rather than a headline. Engagement numbers are de-
 * emphasized here because the real value of a registry entry is its
 * tools/skills/capabilities, not its vanity counts.
 *
 * Historical note: this component was previously named `ActivityStrip`.
 * It was renamed to `EngagementStrip` so the "Activity" label could be
 * reclaimed by the per-entry lifecycle feed (creation, publish, deprecate,
 * etc.) which is a more meaningful use of that word.
 */

import { Eye, Download, Sparkles, RefreshCw } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { SectionHeader } from './section-header'

interface EngagementStripProps {
  viewCount: number
  copyCount: number
  createdAt: string
  updatedAt: string
}

export function EngagementStrip({
  viewCount,
  copyCount,
  createdAt,
  updatedAt,
}: EngagementStripProps) {
  return (
    <section className="space-y-2">
      <SectionHeader title="Engagement" />
      <div className="flex items-center gap-x-6 gap-y-2 text-xs text-muted-foreground flex-wrap px-1">
        <span className="flex items-center gap-1.5">
          <Eye className="h-3.5 w-3.5" />
          <span className="tabular-nums font-medium text-foreground">
            {viewCount.toLocaleString()}
          </span>
          {viewCount === 1 ? 'view' : 'views'}
        </span>
        <span className="flex items-center gap-1.5">
          <Download className="h-3.5 w-3.5" />
          <span className="tabular-nums font-medium text-foreground">
            {copyCount.toLocaleString()}
          </span>
          {copyCount === 1 ? 'install' : 'installs'}
        </span>
        <span className="flex items-center gap-1.5">
          <Sparkles className="h-3.5 w-3.5" />
          Created {formatDate(createdAt)}
        </span>
        <span className="flex items-center gap-1.5">
          <RefreshCw className="h-3.5 w-3.5" />
          Updated {formatDate(updatedAt)}
        </span>
      </div>
    </section>
  )
}
