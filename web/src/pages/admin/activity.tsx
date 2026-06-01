import { useInfiniteQuery } from '@tanstack/react-query'
import { Activity as ActivityIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ActivityTimeline } from '@/components/admin/activity-timeline'
import { useAuthClient } from '@/lib/api-client'
import { usePublisher } from '@/auth/PublisherContext'

const PAGE = 30

// AdminActivity is the full, paginated activity feed for the selected publisher
// — the expanded form of the Overview's recent-activity timeline. Any member
// (Viewer and up) can read it; the endpoint enforces that.
export default function AdminActivity() {
  const { currentSlug } = usePublisher()
  const api = useAuthClient()

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isPending } = useInfiniteQuery({
    queryKey: ['publisher-activity-feed', currentSlug],
    queryFn: ({ pageParam }) =>
      api
        .GET('/api/v1/publishers/{slug}/activity', {
          params: { path: { slug: currentSlug! }, query: { limit: PAGE, cursor: pageParam || undefined } },
        })
        .then((r) => r.data),
    initialPageParam: '',
    getNextPageParam: (last) => last?.next_cursor || undefined,
    enabled: !!currentSlug,
  })

  if (!currentSlug) {
    return (
      <div className="max-w-3xl mx-auto">
        <EmptyState
          icon={<ActivityIcon className="h-10 w-10" />}
          title="Select a publisher"
          description="Pick a publisher from the switcher to see its activity."
        />
      </div>
    )
  }

  const events = data?.pages.flatMap((p) => p?.items ?? []) ?? []

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Activity</h1>
        <p className="text-muted-foreground mt-1 text-sm">
          Everything that happened in <span className="font-mono">{currentSlug}</span>, newest first.
        </p>
      </div>

      {isPending ? (
        <div className="space-y-3">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-10 rounded-md" />
          ))}
        </div>
      ) : events.length === 0 ? (
        <EmptyState
          icon={<ActivityIcon className="h-8 w-8" />}
          title="No activity yet"
          description="Authoring, reviews, and visibility changes will show up here."
        />
      ) : (
        <>
          <ActivityTimeline events={events} />
          {hasNextPage && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" onClick={() => fetchNextPage()} disabled={isFetchingNextPage}>
                {isFetchingNextPage ? 'Loading…' : 'Load more'}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
