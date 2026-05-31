import { Building2 } from 'lucide-react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { usePublisher } from '@/auth/PublisherContext'

// Sentinel for the Server-Admin "All publishers" scope. Radix Select forbids an
// empty-string value, and null can't be a SelectItem value, so the global scope
// (currentSlug === null) maps to this constant on the wire.
const ALL = '__all__'

// PublisherSwitcher scopes the admin area to one publisher. It adapts to what
// the caller can reach: nothing to switch renders a hint or a static label, so
// the dropdown only appears when there's a real choice to make.
export function PublisherSwitcher() {
  const { publishers, isServerAdmin, currentSlug, setCurrent, isLoading } = usePublisher()

  if (isLoading) {
    return <div className="h-9 animate-pulse rounded-md bg-muted/60" aria-hidden="true" />
  }

  // A member with no publishers (and not a Server Admin) has nothing to scope.
  if (publishers.length === 0 && !isServerAdmin) {
    return <p className="px-1 text-xs text-muted-foreground">No publishers yet.</p>
  }

  // One publisher, not a Server Admin: a static label reads cleaner than a
  // single-option dropdown.
  if (publishers.length === 1 && !isServerAdmin) {
    const p = publishers[0]
    if (!p) return null
    return (
      <div className="flex items-center gap-2 rounded-md border bg-background px-3 py-2">
        <Building2 className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Publisher</p>
          <p className="truncate text-sm font-medium">{p.name}</p>
        </div>
      </div>
    )
  }

  return (
    <Select value={currentSlug ?? ALL} onValueChange={(v) => setCurrent(v === ALL ? null : v)}>
      <SelectTrigger aria-label="Switch publisher" className="gap-2">
        <Building2 className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {isServerAdmin && (
          <SelectItem key="__all__" value={ALL}>
            All publishers
          </SelectItem>
        )}
        {isServerAdmin && publishers.length > 0 && <SelectSeparator key="__sep__" />}
        {publishers.map((p) => (
          <SelectItem key={p.slug} value={p.slug}>
            {p.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
