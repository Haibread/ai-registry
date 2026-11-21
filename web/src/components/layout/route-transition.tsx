import { Outlet, useLocation } from 'react-router-dom'

// Keyed on pathname only, so query-string changes (filters, pagination) don't
// replay the fade.
export function RouteTransition() {
  const { pathname } = useLocation()
  return (
    <div
      key={pathname}
      data-testid="route-transition"
      className="animate-in fade-in-0 duration-200 fill-mode-both motion-reduce:animate-none"
    >
      <Outlet />
    </div>
  )
}
