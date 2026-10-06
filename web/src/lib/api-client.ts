import { useMemo } from 'react'
import createClient from 'openapi-fetch'
import type { paths } from './schema'
import { authFetch } from '@/auth/tokens'
import { useAuth } from '@/auth/AuthContext'

/**
 * Public client — plain fetch with no Authorization header, so the response is
 * the public catalog whoever is signed in. Admin/private reads go through
 * useAuthClient; catalog pages that may show a member their unpublished
 * entries go through useCatalogClient.
 */
export function getPublicClient() {
  return createClient<paths>({ baseUrl: '' })
}

/**
 * Hook returning an openapi-fetch client that carries the bearer access token
 * (via authFetch, which also refreshes once on a 401 and retries). If the
 * request still 401s after a refresh attempt — and it isn't /me, which
 * AuthContext owns — we dispatch `auth:unauthorized` so AuthContext flips the UI
 * to signed-out.
 */
export function useAuthClient() {
  return useMemo(() => {
    const client = createClient<paths>({ baseUrl: '', fetch: authFetch })
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

/**
 * Client for catalog pages that a publisher's members should also see while
 * the entry is private or unpublished: carries the bearer token when signed
 * in, anonymous otherwise. `viewer` belongs in the query key so a cached
 * response is never served to another audience after a sign-in or sign-out;
 * `ready` is false until the session is known, so the first fetch does not go
 * out anonymous and 404 on a private entry.
 */
export function useCatalogClient() {
  const { me, isAuthenticated, authLoading } = useAuth()
  const authClient = useAuthClient()
  return {
    api: isAuthenticated ? authClient : getPublicClient(),
    viewer: isAuthenticated ? (me?.user_id ?? 'member') : 'anonymous',
    ready: !authLoading,
  }
}
