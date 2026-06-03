import { Navigate, Outlet } from 'react-router-dom'
import { usePermissions } from '@/auth/useMe'

// RequireServerAdmin gates the cross-publisher management surfaces (users,
// groups, publishers, global grants, reports, audit, API keys) that are
// reserved for Server Admins. It is a pathless layout route: its children
// render through <Outlet/> only when the caller is a Server Admin.
//
// This is defence-in-depth, not the primary control — every Server-Admin API
// is enforced server-side (auth.RequireAdmin). The guard exists so a
// non-admin who navigates directly to a Server-Admin URL (bookmark, typed
// path) is redirected to the dashboard instead of rendering a shell that then
// 403s on every request.
export function RequireServerAdmin() {
  const { isLoading, isServerAdmin } = usePermissions()

  if (isLoading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Loading…</p>
      </div>
    )
  }

  // Authenticated (RequireAuth already passed) but not a Server Admin → bounce
  // to the admin dashboard, which any authenticated principal may view.
  if (!isServerAdmin) {
    return <Navigate to="/admin" replace />
  }

  return <Outlet />
}
