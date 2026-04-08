import { Skeleton } from "@/components/ui/skeleton"

/** Fallback shown while FilterBar hydrates. Matches the FilterBar's layout. */
export function FilterBarSkeleton() {
  return (
    <div className="flex flex-wrap gap-2 items-center" aria-hidden="true">
      <Skeleton className="h-9 flex-1 min-w-[200px] max-w-xs rounded-md" />
      <Skeleton className="h-9 w-36 rounded-md" />
      <Skeleton className="h-9 w-32 rounded-md" />
      <Skeleton className="h-9 w-16 rounded-md" />
    </div>
  )
}
