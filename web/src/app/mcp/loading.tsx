import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { FilterBarSkeleton } from "@/components/ui/filter-bar-skeleton"
import { CardGridSkeleton } from "@/components/ui/card-grid-skeleton"
import { Skeleton } from "@/components/ui/skeleton"

export default function Loading() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <div>
          <Skeleton className="h-8 w-48 rounded" />
          <Skeleton className="h-4 w-72 rounded mt-2" />
        </div>
        <FilterBarSkeleton />
        <CardGridSkeleton count={6} />
      </main>
      <Footer />
    </div>
  )
}
