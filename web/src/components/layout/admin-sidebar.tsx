import { Link, useLocation } from 'react-router-dom'
import { LayoutDashboard, Users, Server, Bot, Key, Flag, Activity } from 'lucide-react'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true },
  { to: '/admin/publishers', label: 'Publishers', icon: Users },
  { to: '/admin/mcp', label: 'MCP Servers', icon: Server },
  { to: '/admin/agents', label: 'Agents', icon: Bot },
  { to: '/admin/reports', label: 'Reports', icon: Flag },
  { to: '/admin/audit', label: 'Activity', icon: Activity },
  { to: '/admin/api-keys', label: 'API Keys', icon: Key },
]

interface AdminSidebarProps {
  pathname?: string
}

export function AdminSidebar({ pathname: pathnameProp }: AdminSidebarProps = {}) {
  const location = useLocation()
  const pathname = pathnameProp ?? location.pathname

  return (
    <aside className="w-56 shrink-0 border-r bg-muted/30 min-h-[calc(100vh-3.5rem)]">
      <nav className="flex flex-col gap-1 p-3">
        {navItems.map(({ to, label, icon: Icon, exact }) => {
          const active = exact ? pathname === to : pathname.startsWith(to)
          return (
            <Link
              key={to}
              to={to}
              className={cn(
                'flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                active
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
