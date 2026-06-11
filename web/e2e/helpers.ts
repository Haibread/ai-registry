import type { Page, APIResponse } from '@playwright/test'

/**
 * Authenticated API helpers.
 *
 * Auth is a registry-issued bearer token. The SPA persists the access token in
 * localStorage (captured by storageState, e2e/.auth/<role>.json). page.request
 * does NOT attach the bearer header automatically, so these helpers read the
 * stored access token and add `Authorization: Bearer ...`.
 *
 * They deliberately do NOT call /auth/refresh: the refresh token is single-use
 * with server-side reuse detection, and the same storageState is shared across
 * every test — rotating it once would revoke the lineage and break the rest of
 * the run. The CI stack uses a long ACCESS_TOKEN_TTL so the stored access token
 * stays valid for the whole suite.
 */
const tokenCache = new WeakMap<Page, string>()

async function accessToken(page: Page): Promise<string> {
  const cached = tokenCache.get(page)
  if (cached) return cached

  // localStorage is only injected once the page is on the app origin; hydrate by
  // visiting the SPA when the test hasn't navigated yet (API-only specs).
  if (!page.url().startsWith('http')) {
    await page.goto('/')
  }
  const token = await page.evaluate(() => window.localStorage.getItem('ai_registry_access'))
  if (!token) {
    throw new Error('e2e helper: no access token in localStorage — storageState not applied or login setup failed')
  }
  tokenCache.set(page, token)
  return token
}

async function authHeaders(page: Page, extra?: Record<string, string>): Promise<Record<string, string>> {
  return { Authorization: `Bearer ${await accessToken(page)}`, ...extra }
}

export async function apiPost(page: Page, path: string, data: unknown): Promise<APIResponse> {
  return page.request.post(path, {
    headers: await authHeaders(page, { 'Content-Type': 'application/json' }),
    data,
  })
}

export async function apiGet(page: Page, path: string): Promise<APIResponse> {
  return page.request.get(path, { headers: await authHeaders(page) })
}

export async function apiPut(page: Page, path: string, data: unknown): Promise<APIResponse> {
  return page.request.put(path, {
    headers: await authHeaders(page, { 'Content-Type': 'application/json' }),
    data,
  })
}

export async function apiPatch(page: Page, path: string, data: unknown): Promise<APIResponse> {
  return page.request.patch(path, {
    headers: await authHeaders(page, { 'Content-Type': 'application/json' }),
    data,
  })
}

export async function apiDelete(page: Page, path: string): Promise<APIResponse> {
  return page.request.delete(path, { headers: await authHeaders(page) })
}

/**
 * Confirms the app's shared <dialog>-based ConfirmDialog (which replaced
 * window.confirm) by clicking its confirm button and waiting for the dialog
 * to close. Use right after clicking an action that asks for confirmation.
 */
export async function confirmDialog(page: Page, buttonName: string | RegExp): Promise<void> {
  const dialog = page.locator('dialog[open]')
  await dialog.getByRole('button', { name: buttonName }).click()
  await dialog.waitFor({ state: 'hidden' })
}
