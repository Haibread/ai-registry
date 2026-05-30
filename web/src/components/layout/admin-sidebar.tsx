import { Link, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { LayoutDashboard, Users, UsersRound, UserCog, Shield, Server, Bot, Key, Flag, Activity, ClipboardCheck } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { usePermissions, type Permissions } from '@/auth/useMe'

// `requires` gates each nav entry by role (ADR 0006):
//   - 'always'      → any authenticated admin-area user (the resource lists are
//                     themselves mine-scoped, so an author sees only their own).
//   - 'reviewer'    → can review on at least one publisher (or Server Admin).
//   - 'serverAdmin' → Server-Admin-only management surfaces.
type NavRequirement = 'always' | 'reviewer' | 'serverAdmin'

const navItems: {
  to: string
  label: string
  icon: typeof LayoutDashboard
  exact?: boolean
  badge?: 'review'
  requires: NavRequirement
}[] = [
  { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true, requires: 'always' },
  { to: '/admin/review', label: 'Review queue', icon: ClipboardCheck, badge: 'review', requires: 'reviewer' },
  { to: '/admin/publishers', label: 'Publishers', icon: Users, requires: 'serverAdmin' },
  { to: '/admin/groups', label: 'Groups', icon: UsersRound, requires: 'serverAdmin' },
  { to: '/admin/users', label: 'Users', icon: UserCog, requires: 'serverAdmin' },
  { to: '/admin/grants', label: 'Global grants', icon: Shield, requires: 'serverAdmin' },
  { to: '/admin/mcp', label: 'MCP Servers', icon: Server, requires: 'always' },
  { to: '/admin/agents', label: 'Agents', icon: Bot, requires: 'always' },
  { to: '/admin/reports', label: 'Reports', icon: Flag, requires: 'serverAdmin' },
  { to: '/admin/audit', label: 'Activity', icon: Activity, requires: 'serverAdmin' },
  { to: '/admin/api-keys', label: 'API Keys', icon: Key, requires: 'serverAdmin' },
]

function navItemVisible(requires: NavRequirement, perms: Permissions): boolean {
  switch (requires) {
    case 'always':
      return true
    case 'reviewer':
      return perms.isReviewerAnywhere
    case 'serverAdmin':
      return perms.isServerAdmin
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

// useReviewQueueCount returns a small integer (capped at 99) representing
// the number of pending items the reviewer has waiting. Returns null while
// the request is in flight or the user is not authenticated. Refetches on
// a 30-second interval so a reviewer who leaves the tab open notices new
// submissions without a hard refresh.
function useReviewQueueCount(): number | null {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const { data } = useQuery({
    queryKey: ['admin-review-queue-count'],
    queryFn: async () => {
      // Limit 99 + check next_cursor — if the queue overflows we display
      // "99+". Larger limits would cost more bandwidth on every admin
      // page load for no UX gain.
      const r = await api.GET('/api/v1/review-queue', {
        params: { query: { limit: 99 } },
      })
      const items = r.data?.items ?? []
      return { count: items.length, hasMore: !!r.data?.next_cursor }
    },
    enabled: !!accessToken,
    refetchInterval: 30_000,
  })
  if (!data) return null
  return data.hasMore ? 100 : data.count
}

function ReviewQueueBadge() {
  const count = useReviewQueueCount()
  if (count === null || count === 0) return null
  const display = count >= 100 ? '99+' : String(count)
  return (
    <span
      aria-label={`${display} item${count === 1 ? '' : 's'} pending review`}
      className="ml-auto inline-flex items-center justify-center rounded-full bg-primary px-2 py-0.5 text-[10px] font-semibold text-primary-foreground tabular-nums"
    >
      {display}
    </span>
  )
}

export function AdminSidebar({ pathname: pathnameProp, mobile, onNavigate }: AdminSidebarProps = {}) {
  const location = useLocation()
  const pathname = pathnameProp ?? location.pathname
  const perms = usePermissions()
  const visibleItems = navItems.filter((item) => navItemVisible(item.requires, perms))

  return (
    <aside
      className={cn(
        'shrink-0 border-r bg-muted/30',
        mobile
          ? 'block w-full h-full'
          : 'hidden md:block w-56 min-h-[calc(100vh-3.5rem)]',
      )}
    >
      <nav className="flex flex-col gap-1 p-3">
        {visibleItems.map(({ to, label, icon: Icon, exact, badge }) => {
          const active = exact ? pathname === to : pathname.startsWith(to)
          return (
            <Link
              key={to}
              to={to}
              onClick={onNavigate}
              className={cn(
                'flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                active
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
              )}
            >
              <Icon className="h-4 w-4" />
              <span className="truncate">{label}</span>
              {badge === 'review' && <ReviewQueueBadge />}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
