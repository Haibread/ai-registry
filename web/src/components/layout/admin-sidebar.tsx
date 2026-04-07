import Link from "next/link"
import { LayoutDashboard, Users, Server, Bot, Key } from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { href: "/admin", label: "Dashboard", icon: LayoutDashboard, exact: true },
  { href: "/admin/publishers", label: "Publishers", icon: Users },
  { href: "/admin/mcp", label: "MCP Servers", icon: Server },
  { href: "/admin/agents", label: "Agents", icon: Bot },
  { href: "/admin/api-keys", label: "API Keys", icon: Key },
]

interface AdminSidebarProps {
  pathname: string
}

export function AdminSidebar({ pathname }: AdminSidebarProps) {
  return (
    <aside className="w-56 shrink-0 border-r bg-muted/30 min-h-[calc(100vh-3.5rem)]">
      <nav className="flex flex-col gap-1 p-3">
        {navItems.map(({ href, label, icon: Icon, exact }) => {
          const active = exact ? pathname === href : pathname.startsWith(href)
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:bg-background/60 hover:text-foreground"
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
