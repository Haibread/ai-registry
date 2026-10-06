import type { ReactNode } from 'react'
import { TooltipInfo } from '@/components/ui/tooltip-info'

export interface DetailFact {
  label: string
  value: ReactNode
  /** Explains the field; rendered as an info tooltip next to the label. */
  tooltip?: string | null | undefined
}

interface DetailFactsProps {
  title?: string
  facts: DetailFact[]
}

/** The "Details" card of a detail page's side column: label / value pairs. */
export function DetailFacts({ title = 'Details', facts }: DetailFactsProps) {
  return (
    <section className="space-y-3 rounded-xl border bg-card p-4" aria-label={title}>
      <h2 className="text-sm font-semibold">{title}</h2>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2.5 text-sm">
        {facts.map((f) => (
          <div key={f.label} className="contents">
            <dt className="flex items-center gap-1 text-muted-foreground">
              {f.label}
              {f.tooltip && <TooltipInfo content={f.tooltip} />}
            </dt>
            <dd className="min-w-0 break-words">{f.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
