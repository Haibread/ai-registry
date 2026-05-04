import { useEffect, useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { Menu, X } from 'lucide-react'
import { AdminSidebar } from '@/components/layout/admin-sidebar'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/auth/AuthContext'

export default function AdminLayout() {
  const { logout, user } = useAuth()
  const location = useLocation()
  const [mobileOpen, setMobileOpen] = useState(false)

  // Auto-close the mobile drawer on route changes — otherwise tapping a
  // nav link leaves the drawer hovering over the new page.
  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  // Lock the body scroll while the drawer is open so backdrop swipes don't
  // bleed through to the underlying page on iOS.
  useEffect(() => {
    if (!mobileOpen) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prev
    }
  }, [mobileOpen])

  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-50 border-b bg-background h-14 flex items-center px-4 md:px-6 gap-2 md:gap-4">
        <Button
          variant="ghost"
          size="icon"
          className="md:hidden -ml-2"
          aria-label={mobileOpen ? 'Close navigation menu' : 'Open navigation menu'}
          aria-expanded={mobileOpen}
          onClick={() => setMobileOpen((v) => !v)}
        >
          {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </Button>
        <Link to="/" className="flex items-center gap-2 font-semibold text-sm min-w-0">
          <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">AI</div>
          <span className="truncate">Registry</span>
        </Link>
        <span className="text-muted-foreground text-sm hidden sm:inline">/</span>
        <span className="text-sm font-medium hidden sm:inline">Admin</span>
        <div className="ml-auto flex items-center gap-2 md:gap-3 min-w-0">
          <ThemeToggle />
          <span
            className="text-sm text-muted-foreground hidden lg:block truncate max-w-[14rem]"
            title={user?.profile?.email}
          >
            {user?.profile?.email}
          </span>
          <Button variant="ghost" size="sm" onClick={logout}>Sign out</Button>
        </div>
      </header>

      {/* Mobile drawer — fixed-position slide-in over the main content. The
          desktop AdminSidebar in the row below stays hidden md:block, so on
          mobile this is the ONLY way to reach the nav. */}
      {mobileOpen && (
        <>
          <button
            type="button"
            aria-label="Close navigation overlay"
            className="fixed inset-0 top-14 z-40 bg-background/80 backdrop-blur-sm md:hidden"
            onClick={() => setMobileOpen(false)}
          />
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Admin navigation"
            className="fixed top-14 left-0 z-50 h-[calc(100vh-3.5rem)] w-64 border-r bg-background shadow-xl md:hidden overflow-y-auto"
          >
            <AdminSidebar mobile onNavigate={() => setMobileOpen(false)} />
          </div>
        </>
      )}

      <div className="flex flex-1 min-w-0">
        <AdminSidebar />
        <main className="flex-1 p-4 md:p-6 overflow-x-auto min-w-0"><Outlet /></main>
      </div>
    </div>
  )
}
