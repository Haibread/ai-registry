import type { Page } from '@playwright/test'

/**
 * Extracts the oidc-client-ts Bearer token from localStorage.
 * oidc-client-ts stores the user object under a key like
 * "oidc.user:<authority>:<client_id>".
 */
export async function getAccessToken(page: Page): Promise<string> {
  const token = await page.evaluate(() => {
    const key = Object.keys(localStorage).find(k => k.startsWith('oidc.user:'))
    if (!key) return ''
    try {
      return (JSON.parse(localStorage.getItem(key)!) as { access_token?: string }).access_token ?? ''
    } catch {
      return ''
    }
  })
  if (!token) throw new Error('No access token found in localStorage — is the admin session loaded?')
  return token
}

/**
 * Makes an authenticated API call using the session Bearer token.
 * Use this instead of page.request.* for admin-only endpoints.
 */
export async function apiPost(page: Page, path: string, data: unknown) {
  const token = await getAccessToken(page)
  return page.request.post(path, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data,
  })
}

export async function apiGet(page: Page, path: string) {
  const token = await getAccessToken(page)
  return page.request.get(path, {
    headers: { Authorization: `Bearer ${token}` },
  })
}
