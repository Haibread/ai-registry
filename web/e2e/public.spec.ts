/**
 * public.spec.ts
 *
 * End-to-end tests for the public (unauthenticated) UI and visibility
 * enforcement:
 *
 *  1. Homepage renders without errors.
 *  2. MCP listing is accessible without auth.
 *  3. Agent listing is accessible without auth.
 *  4. Private entries are NOT visible in the public listing.
 *  5. Admin routes redirect unauthenticated visitors to the homepage (RequireAuth
 *     sends them to '/' when no access token is present).
 */

import { test, expect } from '@playwright/test'

test.describe('Public: Homepage', () => {
  test('renders successfully', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/AI Registry|registry/i)
    // No error banners.
    await expect(page.getByText(/500|Internal Server Error/i)).not.toBeVisible()
  })
})

test.describe('Public: MCP Servers listing', () => {
  test('page loads and shows server cards or empty state', async ({ page }) => {
    await page.goto('/mcp')
    await page.waitForLoadState('networkidle')

    // Servers render as Card components inside a div.grid — each card is a div
    // with rounded-lg border.  If the registry is empty the page shows a
    // well-known empty-state message instead.
    const cards = page.locator('.grid > .rounded-lg')
    const emptyMsg = page.getByText(/No public MCP servers yet\.|No servers match your filters\./)

    const cardsCount = await cards.count()
    const emptyCount = await emptyMsg.count()
    expect(cardsCount + emptyCount).toBeGreaterThan(0)
  })

  test('private servers do not appear in the public listing', async ({ page }) => {
    await page.goto('/mcp')
    await page.waitForLoadState('networkidle')
    // The "private" visibility badge must not appear on a public-facing page.
    const privateLabels = await page.locator('text=private').count()
    expect(privateLabels).toBe(0)
  })
})

test.describe('Public: Agents listing', () => {
  test('page loads and shows agent cards or empty state', async ({ page }) => {
    await page.goto('/agents')
    // Wait for the heading so we know React has rendered the page.
    await expect(page.locator('h1')).toBeVisible({ timeout: 10_000 })

    const cards = page.locator('.grid > .rounded-lg')
    const emptyMsg = page.getByText(/No public agents yet\.|No agents match your filters\./)

    // Wait for either a card or the empty-state message — whichever appears first.
    await expect(cards.first().or(emptyMsg)).toBeVisible({ timeout: 10_000 })
  })
})

test.describe('Public: Auth enforcement on admin routes', () => {
  // RequireAuth redirects to '/' (homepage) when no access token is present.
  // React Router handles this client-side — there is no HTTP-level redirect.
  // We wait for the URL to change rather than for networkidle, because the OIDC
  // library makes background requests that prevent networkidle from resolving.

  test('GET /admin redirects unauthenticated visitors away from admin', async ({ page }) => {
    await page.goto('/admin')
    await page.waitForURL(url => !url.href.includes('/admin'), { timeout: 10_000 })
    expect(page.url()).not.toMatch(/\/admin/)
  })

  test('GET /admin/mcp redirects unauthenticated visitors', async ({ page }) => {
    await page.goto('/admin/mcp')
    await page.waitForURL(url => !url.href.includes('/admin/mcp'), { timeout: 10_000 })
    expect(page.url()).not.toMatch(/\/admin\/mcp/)
  })

  test('GET /admin/agents redirects unauthenticated visitors', async ({ page }) => {
    await page.goto('/admin/agents')
    await page.waitForURL(url => !url.href.includes('/admin/agents'), { timeout: 10_000 })
    expect(page.url()).not.toMatch(/\/admin\/agents/)
  })
})
