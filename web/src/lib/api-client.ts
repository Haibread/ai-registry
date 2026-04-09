import createClient from 'openapi-fetch'
import type { paths } from './schema'

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
