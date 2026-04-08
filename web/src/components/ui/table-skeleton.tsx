import { Skeleton } from "@/components/ui/skeleton"

/** Skeleton table that matches the admin list table layout. */
export function TableSkeleton({ rows = 8, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <div className="rounded-md border overflow-hidden" aria-hidden="true">
      {/* Header */}
      <div className="border-b bg-muted/50 px-4 py-3 grid gap-4" style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}>
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton key={i} className="h-4 w-20 rounded" />
        ))}
      </div>
      {/* Rows */}
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="border-b last:border-0 px-4 py-3 grid gap-4"
          style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}
        >
          {Array.from({ length: cols }).map((_, j) => (
            <Skeleton
              key={j}
              className={`h-4 rounded ${j === 0 ? "w-32" : j === cols - 1 ? "w-14" : "w-24"}`}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
