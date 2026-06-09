/**
 * ux-edge-cases.spec.ts
 *
 * Failure-path and permission-gating coverage that the happy-path journeys
 * don't reach. Several of these use Playwright request interception to force
 * deterministic API failures (500s) so the UI's error handling is actually
 * exercised:
 *
 *   1. A failed list fetch renders the error state (not a silent empty list).
 *   2. Invalid JSON in the New version form is rejected client-side.
 *   3. A failed detail action surfaces the server message via a toast.
 *   4. A partially-failing bulk action reports "N of M succeeded".
 *   5. An editor (not admin) sees Edit but not the Server-Admin hard delete.
 *
 * Interception note: page.route only intercepts the SPA's own fetches, not the
 * APIRequestContext used by the api* helpers — so seeding and out-of-band
 * assertions still hit the real server.
 *
 * Run: npm run test:e2e -- --project=ux-edge-cases
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiGet } from './helpers'

const RUN = Date.now().toString(36)

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin' | 'author'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  await page.goto('/')
  return page
}

async function seedPublisher(page: Page, key: string): Promise<{ slug: string; name: string }> {
  const slug = `e2e-edge-${key}-${RUN}`
  const name = `E2E Edge ${key} ${RUN}`
  const res = await apiPost(page, '/api/v1/publishers', { slug, name })
  expect(res.status(), `create publisher ${slug}: ${await res.text()}`).toBe(201)
  return { slug, name }
}

// ── 1. List fetch failure → error state ────────────────────────────────────────

test.describe('A failed list fetch shows the error state', () => {
  test('forcing the MCP list endpoint to 500 renders the error surface', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')

    // Fail only the collection endpoint (not the /{ns}/{slug} detail routes).
    await page.route(
      (url) => url.pathname === '/api/v1/mcp/servers',
      (route) =>
        route.fulfill({
          status: 500,
          contentType: 'application/problem+json',
          body: JSON.stringify({ title: 'boom', detail: 'simulated failure' }),
        }),
    )

    await page.goto('/admin/mcp')
    await expect(page.getByText(/failed to load mcp servers/i)).toBeVisible({ timeout: 10_000 })
  })
})

// ── 2. Invalid JSON in the New version form is rejected client-side ─────────────

test.describe('New version form validates JSON fields', () => {
  test('invalid capabilities JSON shows an error and creates no version', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'json')
    const slug = `srv-${RUN}`
    const r = await apiPost(page, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'JSON Srv' })
    expect(r.status()).toBe(201)

    await page.goto(`/admin/mcp/${pub.slug}/${slug}`)
    await page.getByRole('button', { name: 'New version' }).click()
    await page.fill('input[name="version"]', '1.0.0')
    await page.fill('textarea[name="capabilities"]', '{ not valid json')
    await page.getByRole('button', { name: 'Create version' }).click()

    // The form surfaces the parse error inline and does not create the version.
    await expect(page.getByText(/capabilities must be valid json/i)).toBeVisible({ timeout: 10_000 })
    const versions = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions`)
    const body = await versions.json()
    expect((body.items ?? []).length).toBe(0)
  })
})

// ── 3. Detail action failure → toast ───────────────────────────────────────────

test.describe('A failed detail action surfaces the error', () => {
  test('a 500 on the visibility toggle shows an error toast', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'toast')
    const slug = `srv-${RUN}`
    // Seed a published server so "Make public" is enabled.
    expect((await apiPost(page, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'Toast Srv' })).status()).toBe(201)
    expect((await apiPost(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions`, {
      version: '1.0.0',
      runtime: 'stdio',
      protocol_version: '2025-03-26',
      packages: [{ registryType: 'npm', identifier: '@e2e/toast', version: '1.0.0', transport: { type: 'stdio' } }],
    })).status()).toBe(201)
    expect((await apiPost(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0/publish`, {})).status()).toBe(200)

    await page.goto(`/admin/mcp/${pub.slug}/${slug}`)
    await expect(page.getByText('published').first()).toBeVisible({ timeout: 10_000 })

    // Fail the visibility POST the SPA sends.
    await page.route(
      (url) => url.pathname === `/api/v1/mcp/servers/${pub.slug}/${slug}/visibility`,
      (route) =>
        route.fulfill({
          status: 500,
          contentType: 'application/problem+json',
          body: JSON.stringify({ title: 'nope', detail: 'visibility service down' }),
        }),
    )

    await page.getByRole('button', { name: /make public/i }).click()
    // The toast carries the server's RFC 7807 detail.
    await expect(page.getByText(/visibility service down/i)).toBeVisible({ timeout: 10_000 })
  })
})

// ── 4. Bulk action partial failure → "N of M succeeded" ────────────────────────

test.describe('A partially-failing bulk action reports progress', () => {
  test('bulk delete where one item 500s reports 1 of 2 succeeded', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'bulk')
    const slugA = `bk-a-${RUN}`
    const slugB = `bk-b-${RUN}`
    for (const s of [slugA, slugB]) {
      expect((await apiPost(page, '/api/v1/mcp/servers', { namespace: pub.slug, slug: s, name: `Bulk ${s}` })).status()).toBe(201)
    }

    await page.goto(`/admin/mcp?namespace=${pub.slug}`)
    await expect(page.getByText(`Bulk ${slugA}`)).toBeVisible({ timeout: 10_000 })

    // Make the DELETE for slugA fail; slugB passes through to the real server.
    await page.route(
      (url) => url.pathname === `/api/v1/mcp/servers/${pub.slug}/${slugA}`,
      (route) =>
        route.request().method() === 'DELETE'
          ? route.fulfill({ status: 500, contentType: 'application/problem+json', body: '{"title":"boom"}' })
          : route.continue(),
    )

    page.once('dialog', (d) => d.accept())
    await page.getByRole('checkbox', { name: /select all/i }).check()
    await page.getByRole('button', { name: /delete/i }).click()

    await expect(page.getByText(/1 of 2 succeeded/i)).toBeVisible({ timeout: 10_000 })
    // The one that wasn't intercepted is actually gone; the failed one remains.
    expect((await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slugB}`)).status()).toBe(404)
    expect((await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slugA}`)).status()).toBe(200)
  })
})

// ── 5. Editor-not-admin permission gating on the detail page ───────────────────

test.describe('Resource detail gates actions by role', () => {
  test('an editor sees Edit + Request deletion but not the hard Delete', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'rbac')
    const slug = `srv-${RUN}`
    expect((await apiPost(admin, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'RBAC Srv' })).status()).toBe(201)

    // Grant the author's anthropic-labs group Editor (not Admin) on this publisher.
    const grp = await apiGet(admin, '/api/v1/groups/anthropic-labs')
    expect(grp.status()).toBe(200)
    const grant = await apiPost(admin, `/api/v1/publishers/${pub.slug}/grants`, {
      principal_type: 'group',
      principal_id: (await grp.json()).id,
      role: 'editor',
    })
    expect(grant.status(), `grant: ${await grant.text()}`).toBe(201)

    const author = await pageAs(browser, 'author')
    await author.goto(`/admin/mcp/${pub.slug}/${slug}`)

    // Editor affordances are present…
    await expect(author.getByRole('button', { name: /^edit$/i })).toBeVisible({ timeout: 10_000 })
    await expect(author.getByRole('button', { name: /request deletion/i })).toBeVisible()
    // …but the Server-Admin-only hard delete is not.
    await expect(author.getByRole('button', { name: 'Delete', exact: true })).toHaveCount(0)
  })
})
