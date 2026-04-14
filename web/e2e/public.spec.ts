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

test.describe('Public: SPA routing for /agents direct links (regression)', () => {
  // Regression guard: an earlier Vite proxy config forwarded every /agents/*
  // request to the backend so the A2A well-known agent card would resolve.
  // That swallowed the SPA detail route too, so direct-link visits to
  // /agents/{ns}/{slug} came back as a 404 from the Go server. The fix scopes
  // the proxy to /agents/{ns}/{slug}/.well-known/* only. These tests make
  // sure the two routings stay split.

  test('GET /agents returns the SPA HTML, not a backend 404', async ({ page }) => {
    const res = await page.request.get('/agents')
    expect(res.status()).toBe(200)
    const body = await res.text()
    // Vite/nginx serve the same index.html shell; it contains the root div.
    expect(body).toMatch(/<div id="root"/)
  })

  test('GET /agents/{ns}/{slug} returns the SPA HTML', async ({ page }) => {
    // Use a slug that almost certainly does not exist. The point is that the
    // SPA shell must load so React Router can render the "not found" state —
    // the HTTP layer itself must not 404.
    const res = await page.request.get('/agents/does-not-exist/also-missing')
    expect(res.status()).toBe(200)
    const body = await res.text()
    expect(body).toMatch(/<div id="root"/)
  })

  test('GET /agents/{ns}/{slug}/.well-known/agent-card.json still proxies to the backend', async ({ page }) => {
    // The well-known subpath must still reach the backend. A non-existent
    // pair should come back as 404 (from the backend, not from the SPA shell).
    const res = await page.request.get('/agents/does-not-exist/also-missing/.well-known/agent-card.json')
    expect(res.status()).toBe(404)
    // Not the SPA HTML.
    const body = await res.text()
    expect(body).not.toMatch(/<div id="root"/)
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
