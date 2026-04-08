import { FilterBarSkeleton } from "@/components/ui/filter-bar-skeleton"
import { TableSkeleton } from "@/components/ui/table-skeleton"
import { Skeleton } from "@/components/ui/skeleton"

export default function Loading() {
  return (
    <div className="space-y-4 max-w-5xl">
      <div className="flex items-center justify-between">
        <div className="space-y-1.5">
          <Skeleton className="h-7 w-32 rounded" />
          <Skeleton className="h-4 w-24 rounded" />
        </div>
        <Skeleton className="h-9 w-28 rounded" />
      </div>
      <FilterBarSkeleton />
      <TableSkeleton rows={8} cols={6} />
    </div>
  )
}
