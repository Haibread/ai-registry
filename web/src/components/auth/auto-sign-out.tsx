"use client"

import { useEffect } from "react"
import { signOut } from "next-auth/react"

/**
 * Rendered by the admin layout when the session has an unrecoverable error
 * (e.g. `RefreshAccessTokenError`).
 *
 * Because `signOut()` from `next-auth` must be called from a Server Action to
 * properly clear the session cookie, we use the `next-auth/react` variant here,
 * which posts to `/api/auth/signout` from the client and handles CSRF
 * automatically.  The user is redirected to the home page after sign-out.
 *
 * A redirect-only approach (server-side `redirect("/api/auth/signin")`) leaves
 * the stale session cookie in place and can create redirect loops.  This
 * component ensures the cookie is fully cleared before the user re-authenticates.
 */
export function AutoSignOut() {
  useEffect(() => {
    signOut({ callbackUrl: "/" })
  }, [])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-muted-foreground animate-pulse">
        Session expired — signing you out…
      </p>
    </div>
  )
}
