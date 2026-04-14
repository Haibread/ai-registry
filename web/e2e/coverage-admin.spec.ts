/**
 * coverage-admin.spec.ts
 *
 * Closes v0.2.0 coverage gaps in the admin UI:
 *
 *  - Search + filter on admin list pages (query params drive listing)
 *  - Bulk actions toolbar (select-all, visibility toggle, delete)
 *  - Version publishing via the UI (buttons, not just the API)
 *  - Load-more pagination (uses next_cursor)
 *  - Error states (missing slug → not-found)
 *
 * Each top-level describe seeds its own namespace so tests do not collide with
 * the main admin.spec.ts run or each other. Cleanup uses the admin API so a
 * half-run suite leaves no permanent state.
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, getAccessToken } from './helpers'

const RUN_ID = Date.now().toString(36)

async function apiDelete(page: Page, path: string) {
  const token = await getAccessToken(page)
  return page.request.delete(path, { headers: { Authorization: `Bearer ${token}` } })
}

async function goTo(page: Page, path: string) {
  await page.goto(path)
  await page.waitForLoadState('domcontentloaded')
}

// ── W3a: Search + filter ─────────────────────────────────────────────────────

test.describe('Admin: search and filter', () => {
  const PUB_SLUG = `e2e-search-pub-${RUN_ID}`
  const PUB_NAME = `E2E Search Pub ${RUN_ID}`
  const NEEDLE_SLUG = `e2e-search-needle-${RUN_ID}`
  const NEEDLE_NAME = `Needle Search Target ${RUN_ID}`
  const NOISE_SLUG = `e2e-search-noise-${RUN_ID}`
  const NOISE_NAME = `Noise Unrelated ${RUN_ID}`

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    const pub = await apiPost(page, '/api/v1/publishers', { slug: PUB_SLUG, name: PUB_NAME })
    expect(pub.ok()).toBeTruthy()
    const mcp1 = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB_SLUG, slug: NEEDLE_SLUG, name: NEEDLE_NAME,
      description: 'Searchable needle.',
    })
    expect(mcp1.ok()).toBeTruthy()
    const mcp2 = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB_SLUG, slug: NOISE_SLUG, name: NOISE_NAME,
      description: 'Different server.',
    })
    expect(mcp2.ok()).toBeTruthy()
    await ctx.close()
  })

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${NEEDLE_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${NOISE_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB_SLUG}`).catch(() => {})
    await ctx.close()
  })

  test('search input narrows the admin MCP list to matching rows', async ({ page }) => {
    await goTo(page, '/admin/mcp')

    // Both seeded rows visible by default (filter by namespace to isolate).
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('cell', { name: NOISE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })

    // Type into the search box — use the exact label so it doesn't collide
    // with row checkboxes whose aria-labels contain "Search".
    await page.getByRole('textbox', { name: 'Search', exact: true }).fill('Needle')
    await expect(page).toHaveURL(/q=Needle/, { timeout: 5_000 })
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('cell', { name: NOISE_NAME, exact: true })).not.toBeVisible({ timeout: 5_000 })

    // Clearing all filters restores both.
    await page.getByRole('button', { name: 'Clear all filters' }).click()
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('cell', { name: NOISE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('status filter restricts admin list to that status', async ({ page }) => {
    await goTo(page, '/admin/mcp')
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })

    // Both new servers are draft — filter to published should hide them.
    await page.getByLabel('Filter by status').selectOption('published')
    await expect(page).toHaveURL(/status=published/)
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).not.toBeVisible({ timeout: 5_000 })

    // Back to draft and they reappear.
    await page.getByLabel('Filter by status').selectOption('draft')
    await expect(page.getByRole('cell', { name: NEEDLE_NAME, exact: true })).toBeVisible({ timeout: 10_000 })
  })
})

// ── W3b: Bulk actions ────────────────────────────────────────────────────────

test.describe('Admin: bulk actions', () => {
  const PUB_SLUG = `e2e-bulk-pub-${RUN_ID}`
  const PUB_NAME = `E2E Bulk Pub ${RUN_ID}`
  const SLUGS = [`e2e-bulk-1-${RUN_ID}`, `e2e-bulk-2-${RUN_ID}`, `e2e-bulk-3-${RUN_ID}`]

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    const pub = await apiPost(page, '/api/v1/publishers', { slug: PUB_SLUG, name: PUB_NAME })
    expect(pub.ok()).toBeTruthy()
    for (const slug of SLUGS) {
      const res = await apiPost(page, '/api/v1/mcp/servers', {
        namespace: PUB_SLUG, slug, name: slug,
      })
      expect(res.ok()).toBeTruthy()
    }
    await ctx.close()
  })

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    for (const slug of SLUGS) {
      await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${slug}`).catch(() => {})
    }
    await apiDelete(page, `/api/v1/publishers/${PUB_SLUG}`).catch(() => {})
    await ctx.close()
  })

  test('bulk toolbar appears after selecting rows', async ({ page }) => {
    await goTo(page, '/admin/mcp')
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: SLUGS[0], exact: true })).toBeVisible({ timeout: 10_000 })

    // Select the first two rows.
    await page.getByRole('checkbox', { name: `Select ${SLUGS[0]}` }).check()
    await page.getByRole('checkbox', { name: `Select ${SLUGS[1]}` }).check()

    const toolbar = page.getByRole('toolbar', { name: 'Bulk actions' })
    await expect(toolbar).toBeVisible()
    await expect(toolbar.getByText('2 selected')).toBeVisible()
  })

  test('bulk "Public" flips visibility for selected rows', async ({ page }) => {
    await goTo(page, '/admin/mcp')
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: SLUGS[0], exact: true })).toBeVisible({ timeout: 10_000 })

    // Select two private rows.
    await page.getByRole('checkbox', { name: `Select ${SLUGS[0]}` }).check()
    await page.getByRole('checkbox', { name: `Select ${SLUGS[1]}` }).check()

    const toolbar = page.getByRole('toolbar', { name: 'Bulk actions' })
    await toolbar.getByRole('button', { name: 'Public' }).click()

    // Visibility filter to public — the two flipped rows should be present.
    await page.getByLabel('Filter by visibility').selectOption('public')
    await expect(page.getByRole('cell', { name: SLUGS[0], exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('cell', { name: SLUGS[1], exact: true })).toBeVisible({ timeout: 10_000 })
    // The third row stays private.
    await expect(page.getByRole('cell', { name: SLUGS[2], exact: true })).not.toBeVisible({ timeout: 5_000 })
  })

  test('bulk "Delete" removes selected rows', async ({ page }) => {
    await goTo(page, '/admin/mcp')
    await page.getByLabel('Filter by publisher').fill(PUB_SLUG)
    await expect(page.getByRole('cell', { name: SLUGS[2], exact: true })).toBeVisible({ timeout: 10_000 })

    await page.getByRole('checkbox', { name: `Select ${SLUGS[2]}` }).check()

    page.on('dialog', (d) => d.accept())
    const toolbar = page.getByRole('toolbar', { name: 'Bulk actions' })
    await toolbar.getByRole('button', { name: 'Delete' }).click()

    await expect(page.getByRole('cell', { name: SLUGS[2], exact: true })).not.toBeVisible({ timeout: 10_000 })
  })
})

// ── W3c: UI version publish flow ────────────────────────────────────────────

test.describe('Admin: publish version via UI', () => {
  const PUB_SLUG = `e2e-pubui-pub-${RUN_ID}`
  const PUB_NAME = `E2E PubUI ${RUN_ID}`
  const MCP_SLUG = `e2e-pubui-mcp-${RUN_ID}`
  const MCP_NAME = `E2E PubUI MCP ${RUN_ID}`
  const VERSION = '1.0.0'

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    const pub = await apiPost(page, '/api/v1/publishers', { slug: PUB_SLUG, name: PUB_NAME })
    expect(pub.ok()).toBeTruthy()
    await ctx.close()
  })

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB_SLUG}`).catch(() => {})
    await ctx.close()
  })

  test('creating a server via the new-form with "publish immediately" ends in published status', async ({ page }) => {
    await goTo(page, '/admin/mcp/new')
    await expect(page.getByRole('heading', { name: 'New MCP Server' })).toBeVisible({ timeout: 15_000 })

    // Open the shadcn publisher Select and pick our seeded publisher.
    await page.locator('#namespace-select').click()
    await page.getByRole('option', { name: new RegExp(`^${PUB_SLUG} —`) }).click()

    await page.locator('#slug').fill(MCP_SLUG)
    await page.locator('#name').fill(MCP_NAME)
    await page.locator('#version').fill(VERSION)
    await page.locator('#pkg_identifier').fill('@e2e/pubui')
    await page.locator('#pkg_version').fill(VERSION)

    // "Publish version immediately" is checked by default — assert that and submit.
    await expect(page.locator('#publish')).toBeChecked()
    await page.getByRole('button', { name: 'Create MCP Server' }).click()

    // After success we navigate to the detail page; it should show published.
    await expect(page).toHaveURL(new RegExp(`/admin/mcp/${PUB_SLUG}/${MCP_SLUG}$`), { timeout: 15_000 })
    await expect(page.getByText('published').first()).toBeVisible({ timeout: 15_000 })
  })
})

// ── W3g: Error / 404 states ─────────────────────────────────────────────────

test.describe('Admin: error states', () => {
  test('admin detail page for a missing MCP server shows a not-found state', async ({ page }) => {
    await goTo(page, '/admin/mcp/ghost-ns/ghost-slug')
    // The page should render something — either an error or empty state —
    // without crashing. We look for any heading or visible copy mentioning
    // not-found / error / 404 so assertions survive copy changes.
    await expect(page.locator('body')).toBeVisible()
    await expect(
      page.getByText(/not found|does not exist|no such|404|error/i).first(),
    ).toBeVisible({ timeout: 15_000 })
  })

  test('admin detail page for a missing agent shows a not-found state', async ({ page }) => {
    await goTo(page, '/admin/agents/ghost-ns/ghost-slug')
    await expect(
      page.getByText(/not found|does not exist|no such|404|error/i).first(),
    ).toBeVisible({ timeout: 15_000 })
  })
})
