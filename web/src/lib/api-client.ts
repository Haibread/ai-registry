import { useMemo } from 'react'
import createClient from 'openapi-fetch'
import type { paths } from './schema'

// withCredentials sends the registry session cookie on every request (ADR 0006
// amendment, 2026-06-01). Auth is the HttpOnly cookie — there is no bearer token
// in JS — so the client just needs credentials:'include'.
const withCredentials: typeof fetch = (input, init) =>
  fetch(input, { ...init, credentials: 'include' })

/** Public client — read-only public pages. Still sends the cookie if present. */
export function getPublicClient() {
  return createClient<paths>({ baseUrl: '', fetch: withCredentials })
}

/**
 * Hook returning an openapi-fetch client that carries the session cookie. A 401
 * on any request other than /me means the session lapsed mid-use; we dispatch
 * `auth:unauthorized` so AuthContext re-checks and the UI flips to signed-out.
 * /me itself is skipped (AuthContext owns that fetch).
 */
export function useAuthClient() {
  return useMemo(() => {
    const client = createClient<paths>({ baseUrl: '', fetch: withCredentials })
    client.use({
      async onResponse({ response }) {
        if (response.status === 401 && !response.url.endsWith('/api/v1/me')) {
          window.dispatchEvent(new Event('auth:unauthorized'))
        }
        return response
      },
    })
    return client
  }, [])
}
