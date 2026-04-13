import { Link, useLocation } from 'react-router-dom'
import { AlertCircle, BookOpen } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { NavLink } from '@/components/layout/nav-link'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { SearchBar } from '@/components/ui/search-bar'
import { useAuth } from '@/auth/AuthContext'

export function Header() {
  const { accessToken, login, logout, loginError } = useAuth()
  const location = useLocation()
  // The home hero already hosts a big SearchBar — don't duplicate it in the
  // header on `/`, but surface it on every other page so cross-type search is
  // one click away from anywhere.
  const showHeaderSearch = location.pathname !== '/'

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur-sm supports-backdrop-filter:bg-background/60">
      {loginError && (
        <div role="alert" className="flex items-center gap-2 border-b border-destructive/30 bg-destructive/10 px-4 py-2 text-xs text-destructive">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          {loginError}
        </div>
      )}
      <div className="container flex h-14 items-center gap-6">
        <Link to="/" className="flex items-center gap-2 font-semibold shrink-0">
          <div className="flex h-7 w-7 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">
            AI
          </div>
          <span>Registry</span>
        </Link>

        <nav className="flex items-center gap-1">
          <NavLink to="/mcp">
            <ResourceIcon type="mcp-server" />
            MCP Servers
          </NavLink>
          <NavLink to="/agents">
            <ResourceIcon type="agent" />
            Agents
          </NavLink>
          <NavLink to="/getting-started">
            <BookOpen className="h-4 w-4" />
            Getting Started
          </NavLink>
        </nav>

        {showHeaderSearch && (
          <div className="ml-auto flex-1 max-w-sm hidden md:block">
            <SearchBar variant="compact" />
          </div>
        )}

        <div className={`flex items-center gap-2 ${showHeaderSearch ? 'ml-2' : 'ml-auto'}`}>
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
