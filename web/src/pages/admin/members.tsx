import { Link } from 'react-router-dom'
import { Users } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { GrantsSection } from '@/components/admin/grants-section'
import { usePublisher } from '@/auth/PublisherContext'
import { usePermissions } from '@/auth/useMe'

// AdminMembers manages the role grants for the currently selected publisher.
// Scoped via the publisher switcher rather than a URL param, so it follows the
// rest of the admin shell. Reachable only by a publisher Admin (the nav hides
// it otherwise); this also guards a deep link.
export default function AdminMembers() {
  const { currentSlug, current } = usePublisher()
  const perms = usePermissions()

  if (!currentSlug) {
    return (
      <div className="max-w-3xl mx-auto">
        <EmptyState
          icon={<Users className="h-10 w-10" />}
          title="Select a publisher"
          description="Pick a publisher from the switcher to manage its members and roles."
        />
      </div>
    )
  }

  if (!perms.canAdmin(currentSlug)) {
    return (
      <div className="max-w-3xl mx-auto">
        <EmptyState
          icon={<Users className="h-10 w-10" />}
          title="Admin access required"
          description={`You need the Admin role on ${current?.name ?? currentSlug} to manage its members.`}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Members &amp; roles</h1>
        <p className="text-muted-foreground mt-1 text-sm">
          Who can act in <span className="font-mono">{currentSlug}</span>, and how. Grant a role to a
          user or a group.{' '}
          {perms.isServerAdmin && (
            <Link to={`/admin/publishers/${currentSlug}`} className="text-foreground underline-offset-2 hover:underline">
              Open the full publisher page
            </Link>
          )}
        </p>
      </div>

      <GrantsSection publisherSlug={currentSlug} />
    </div>
  )
}
