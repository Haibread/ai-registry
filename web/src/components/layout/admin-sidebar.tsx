import { Link, useLocation } from 'react-router-dom'
import { LayoutDashboard, Users, UsersRound, UserCog, Shield, Plug, Flag, Activity, ScrollText, ClipboardCheck, Settings, Tags } from 'lucide-react'
import { cn } from '@/lib/utils'
import { formatQueueCount, useReviewQueueCount } from '@/hooks/use-review-queue-count'
import { usePermissions, type Permissions } from '@/auth/useMe'
import { usePublisher } from '@/auth/PublisherContext'
import { PublisherSwitcher } from '@/components/admin/publisher-switcher'

// `requires` gates each nav entry:
//   - 'always'          → any authenticated admin-area user (resource lists are
//                         mine-scoped, so an author sees only their own).
//   - 'reviewer'        → can review on at least one publisher (or Server Admin).
//   - 'serverAdmin'     → Server-Admin-only management surfaces.
//   - 'publisherMember' → a publisher is selected (per-publisher pages).
//   - 'publisherAdmin'  → Admin on the selected publisher (or Server Admin).
type NavRequirement = 'always' | 'reviewer' | 'serverAdmin' | 'publisherMember' | 'publisherAdmin'

interface NavItem {
  to: string
  label: string
  icon: typeof LayoutDashboard
  exact?: boolean
  badge?: 'review'
  requires: NavRequirement
}

// Publisher-scoped surfaces: the day-to-day work in the publisher the switcher
// has selected. Resource lists are mine-scoped, so an author sees only theirs.
const publisherNav: NavItem[] = [
  { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true, requires: 'always' },
  { to: '/admin/mcp', label: 'MCP Servers', icon: Plug, requires: 'always' },
  { to: '/admin/review', label: 'Review queue', icon: ClipboardCheck, badge: 'review', requires: 'reviewer' },
  { to: '/admin/activity', label: 'Activity', icon: Activity, requires: 'publisherMember' },
  { to: '/admin/members', label: 'Members', icon: Users, requires: 'publisherAdmin' },
  // Visible to any publisher member; non-admins see it read-only (the page
  // gates editing on canAdmin), so members can review name/contact/slug.
  { to: '/admin/settings', label: 'Settings', icon: Settings, requires: 'publisherMember' },
]

// Server-Admin management surfaces: global, cross-publisher administration,
// hidden from non-admins.
const serverAdminNav: NavItem[] = [
  { to: '/admin/publishers', label: 'Publishers', icon: Users, requires: 'serverAdmin' },
  { to: '/admin/groups', label: 'Groups', icon: UsersRound, requires: 'serverAdmin' },
  { to: '/admin/users', label: 'Users', icon: UserCog, requires: 'serverAdmin' },
  { to: '/admin/grants', label: 'Global grants', icon: Shield, requires: 'serverAdmin' },
  { to: '/admin/tags', label: 'Instance tags', icon: Tags, requires: 'serverAdmin' },
  { to: '/admin/reports', label: 'Reports', icon: Flag, requires: 'serverAdmin' },
  { to: '/admin/audit', label: 'Audit log', icon: ScrollText, requires: 'serverAdmin' },
]

function navItemVisible(requires: NavRequirement, perms: Permissions, currentSlug: string | null): boolean {
  switch (requires) {
    case 'always':
      return true
    case 'reviewer':
      return perms.isReviewerAnywhere
    case 'serverAdmin':
      return perms.isServerAdmin
    case 'publisherMember':
      return currentSlug !== null
    case 'publisherAdmin':
      return currentSlug !== null && perms.canAdmin(currentSlug)
  }
}

interface AdminSidebarProps {
  pathname?: string
  // When true, render as a full-width sidebar suitable for a mobile drawer.
  // The default is the desktop static sidebar (hidden below md).
  mobile?: boolean
  // Notify the parent (e.g. the mobile drawer) when a nav link is clicked,
  // so it can close itself. Ignored on desktop.
  onNavigate?: () => void
}

function ReviewQueueBadge() {
  const queue = useReviewQueueCount()
  if (!queue || queue.count === 0) return null
  const count = queue.count
  const display = formatQueueCount(queue)
  return (
    <span
      aria-label={`${display} item${count === 1 ? '' : 's'} pending review`}
      className="ml-auto inline-flex items-center justify-center rounded-full bg-highlight/15 px-2 py-0.5 text-[11px] font-semibold text-highlight-foreground tabular-nums"
    >
      {display}
    </span>
  )
}

export function AdminSidebar({ pathname: pathnameProp, mobile, onNavigate }: AdminSidebarProps = {}) {
  const location = useLocation()
  const pathname = pathnameProp ?? location.pathname
  const perms = usePermissions()
  const { currentSlug } = usePublisher()
  const publisherItems = publisherNav.filter((item) => navItemVisible(item.requires, perms, currentSlug))
  const serverAdminItems = serverAdminNav.filter((item) => navItemVisible(item.requires, perms, currentSlug))

  const renderItem = ({ to, label, icon: Icon, exact, badge }: NavItem) => {
    const active = exact ? pathname === to : pathname.startsWith(to)
    return (
      <Link
        key={to}
        to={to}
        onClick={onNavigate}
        className={cn(
          // Same focus language as Button (ring, not the UA outline) so
          // keyboard focus looks identical across the chrome.
          'flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors',
          'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ring-offset-background',
          active
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
        )}
      >
        <Icon className="h-4 w-4" />
        <span className="truncate">{label}</span>
        {badge === 'review' && <ReviewQueueBadge />}
      </Link>
    )
  }

  return (
    <aside
      className={cn(
        'shrink-0 border-r bg-muted/30',
        mobile
          ? 'block w-full h-full'
          : 'hidden md:block w-56 min-h-[calc(100vh-3.5rem)]',
      )}
    >
      <div className="p-3 pb-1">
        <PublisherSwitcher />
      </div>
      <nav className="flex flex-col gap-1 px-3 pb-3">
        {publisherItems.map(renderItem)}
        {serverAdminItems.length > 0 && (
          <>
            <p className="mt-4 mb-1 px-3 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              Server admin
            </p>
            {serverAdminItems.map(renderItem)}
          </>
        )}
      </nav>
    </aside>
  )
}
