import { Link } from 'react-router-dom'
import { Server, Bot } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { NavLink } from '@/components/layout/nav-link'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { useAuth } from '@/auth/AuthContext'

export function Header() {
  const { accessToken, login, logout } = useAuth()

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur-sm supports-backdrop-filter:bg-background/60">
      <div className="container flex h-14 items-center gap-6">
        <Link to="/" className="flex items-center gap-2 font-semibold shrink-0">
          <div className="flex h-7 w-7 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">
            AI
          </div>
          <span>Registry</span>
        </Link>

        <nav className="flex items-center gap-1">
          <NavLink to="/mcp">
            <Server className="h-4 w-4" aria-hidden="true" />
            MCP Servers
          </NavLink>
          <NavLink to="/agents">
            <Bot className="h-4 w-4" aria-hidden="true" />
            Agents
          </NavLink>
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          {accessToken ? (
            <>
              <Button variant="ghost" size="sm" asChild>
                <Link to="/admin">Admin</Link>
              </Button>
              <Button variant="outline" size="sm" onClick={logout}>
                Sign out
              </Button>
            </>
          ) : (
            <Button size="sm" onClick={login}>
              Sign in
            </Button>
          )}
        </div>
      </div>
    </header>
  )
}
