import { useMemo } from 'react'
import createClient from 'openapi-fetch'
import type { paths } from './schema'
import { useAuth } from '@/auth/AuthContext'

/** Public unauthenticated client — for read-only public pages. */
export function getPublicClient() {
  return createClient<paths>({ baseUrl: '' })
}

/** Authenticated client — pass the access token from useAuth(). */
export function getAuthClient(token: string) {
  return createClient<paths>({
    baseUrl: '',
    headers: { Authorization: `Bearer ${token}` },
  })
}

/**
 * Hook that returns an openapi-fetch client pre-configured with the current
 * access token. Any 401 response automatically clears the local session so
 * the UI immediately shows the Sign in button without a Keycloak redirect.
 */
export function useAuthClient() {
  const { accessToken, clearSession } = useAuth()

  return useMemo(() => {
    const client = createClient<paths>({ baseUrl: '' })
    client.use({
      async onRequest({ request }) {
        if (accessToken) request.headers.set('Authorization', `Bearer ${accessToken}`)
        return request
      },
      async onResponse({ response }) {
        if (response.status === 401) await clearSession()
        return response
      },
    })
    return client
  }, [accessToken, clearSession])
}
