import { cn } from '@/lib/utils'

/** The newest revision comes first in the API and is the one highlighted. */
export function ProtocolVersionBadges({ versions, className }: { versions: string[]; className?: string }) {
  return (
    <span className={cn('flex flex-wrap gap-1.5', className)}>
      {versions.map((v, i) => (
        <span
          key={v}
          className={cn(
            'rounded-full px-2 py-0.5 font-mono text-xs',
            i === 0 ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground',
          )}
        >
          {v}
        </span>
      ))}
    </span>
  )
}
