/**
 * VersionDiff — compact side-by-side diff of two version records.
 *
 * We don't do character-level diffing. Instead we walk a fixed set of
 * interesting top-level fields and show old → new for any that differ.
 * Object-valued fields (packages, capabilities, skills, …) are stringified
 * via JSON so reviewers can eyeball the change. Equal fields are hidden.
 */

import { Badge } from '@/components/ui/badge'

interface VersionLike {
  version: string
  runtime?: string | null
  protocol_version?: string | null
  checksum?: string | null
  signature?: string | null
  packages?: unknown
  capabilities?: unknown
  skills?: unknown
  auth?: unknown
  status?: string | null
  published_at?: string | null
}

interface VersionDiffProps {
  a: VersionLike
  b: VersionLike
}

const COMPARABLE_FIELDS: (keyof VersionLike)[] = [
  'runtime',
  'protocol_version',
  'checksum',
  'signature',
  'packages',
  'capabilities',
  'skills',
  'auth',
  'status',
]

function stringify(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'string') return v || '—'
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

function fieldsEqual(x: unknown, y: unknown): boolean {
  if (x === y) return true
  if (x == null && y == null) return true
  try {
    return JSON.stringify(x) === JSON.stringify(y)
  } catch {
    return false
  }
}

export function VersionDiff({ a, b }: VersionDiffProps) {
  const changed = COMPARABLE_FIELDS.filter((f) => !fieldsEqual(a[f], b[f]))

  return (
    <div className="rounded-md border p-3 space-y-3 text-sm">
      <div className="flex items-center gap-2">
        <span className="font-medium">Comparing</span>
        <Badge variant="outline" className="font-mono">v{a.version}</Badge>
        <span className="text-muted-foreground">→</span>
        <Badge variant="outline" className="font-mono">v{b.version}</Badge>
      </div>

      {changed.length === 0 ? (
        <p className="text-muted-foreground text-xs">
          No differences in structured fields.
        </p>
      ) : (
        <ul className="divide-y" data-testid="diff-field-list">
          {changed.map((field) => (
            <li key={field} className="py-2 space-y-1">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {field}
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                <pre className="rounded bg-destructive/10 border border-destructive/20 p-2 overflow-x-auto text-xs whitespace-pre-wrap break-words">
                  {stringify(a[field])}
                </pre>
                <pre className="rounded bg-green-500/10 border border-green-500/20 p-2 overflow-x-auto text-xs whitespace-pre-wrap break-words">
                  {stringify(b[field])}
                </pre>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
