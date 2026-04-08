import createClient, { type Middleware } from "openapi-fetch"
import type { paths } from "./schema"
import { auth } from "@/auth"
import { redirect } from "next/navigation"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

/**
 * openapi-fetch middleware that intercepts 401 responses from the backend.
 *
 * A 401 means the backend rejected the bearer token — either it was revoked in
 * Keycloak mid-session, or something caused a clock skew between NextAuth's
 * expiry check and the backend's JWKS validation.  We can't call `signOut()`
 * here (only a Server Action can mutate cookies), so we throw a Next.js
 * redirect instead.  The admin layout's <AutoSignOut> client component will
 * clean up the session cookie on the client side.
 */
const unauthorizedMiddleware: Middleware = {
  async onResponse({ response }) {
    if (response.status === 401) {
      redirect("/api/auth/signin")
    }
    return undefined
  },
}

/**
 * Returns a server-side API client with the current session's access token
 * injected as a Bearer header. Must only be called from Server Components,
 * Route Handlers, or Server Actions.
 *
 * Automatically redirects to the sign-in page when:
 *   • the Keycloak token refresh has already failed (`RefreshAccessTokenError`), or
 *   • the backend returns HTTP 401 for the authenticated request.
 */
export async function getApiClient() {
  const session = await auth()

  // Token refresh failed — redirect before wasting a backend round-trip.
  // The middleware already blocks /admin access on page navigations, but
  // SSR inside a page that is still in-flight may arrive here first.
  if (session?.error === "RefreshAccessTokenError") {
    redirect("/api/auth/signin")
  }

  const headers: Record<string, string> = {}
  if (session?.accessToken) {
    headers["Authorization"] = `Bearer ${session.accessToken}`
  }

  const client = createClient<paths>({ baseUrl: API_URL, headers })
  client.use(unauthorizedMiddleware)
  return client
}

/**
 * Unauthenticated client for public read endpoints.
 * Safe to call from any server context.
 */
export function getPublicClient() {
  return createClient<paths>({ baseUrl: API_URL })
}
