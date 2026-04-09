import { Link, Outlet } from 'react-router-dom'
import { AdminSidebar } from '@/components/layout/admin-sidebar'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/auth/AuthContext'

export default function AdminLayout() {
  const { logout, user } = useAuth()

  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-50 border-b bg-background h-14 flex items-center px-6 gap-4">
        <Link to="/" className="flex items-center gap-2 font-semibold text-sm">
          <div className="flex h-6 w-6 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">AI</div>
          Registry
        </Link>
        <span className="text-muted-foreground text-sm">/</span>
        <span className="text-sm font-medium">Admin</span>
        <div className="ml-auto flex items-center gap-3">
          <ThemeToggle />
          <span className="text-sm text-muted-foreground hidden sm:block">{user?.profile?.email}</span>
          <Button variant="ghost" size="sm" onClick={logout}>Sign out</Button>
        </div>
      </header>
      <div className="flex flex-1">
        <AdminSidebar />
        <main className="flex-1 p-6 overflow-auto"><Outlet /></main>
      </div>
    </div>
  )
}
