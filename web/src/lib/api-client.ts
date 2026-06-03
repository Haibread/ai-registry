import { useMemo } from 'react'
import createClient from 'openapi-fetch'
import type { paths } from './schema'
import { authFetch } from '@/auth/tokens'

/**
 * Public client — read-only public pages. Uses plain fetch with no Authorization
 * header, so public pages always render the public catalog regardless of who is
 * signed in. Admin/private reads go through useAuthClient.
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
