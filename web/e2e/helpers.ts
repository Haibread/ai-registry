import type { Page } from '@playwright/test'

/**
 * Authenticated API helpers.
 *
 * Auth is now a registry session behind an HttpOnly cookie — there is no bearer
 * token and nothing in localStorage to read. The browser context loads the
 * session cookie from its storageState (e2e/.auth/<role>.json), and Playwright's
 * `page.request` shares that same cookie jar, so these calls are authenticated
 * automatically. Use them instead of raw `page.request.*` for admin-only writes.
 */
export async function apiPost(page: Page, path: string, data: unknown) {
  return page.request.post(path, {
    headers: { 'Content-Type': 'application/json' },
    data,
  })
}

export async function apiGet(page: Page, path: string) {
  return page.request.get(path)
}

export async function apiPatch(page: Page, path: string, data: unknown) {
  return page.request.patch(path, {
    headers: { 'Content-Type': 'application/json' },
    data,
  })
}

export async function apiDelete(page: Page, path: string) {
  return page.request.delete(path)
}
