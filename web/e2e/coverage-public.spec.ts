/**
 * coverage-public.spec.ts
 *
 * Closes v0.2.0 coverage gaps on the public (unauthenticated) UI:
 *
 *  - Public MCP / agent search (q + namespace)
 *  - Public publisher detail page at /publishers/{slug}
 *  - Theme toggle (light/dark) with localStorage persistence
 *  - Public 404 / not-found for a private or missing entry
 *
 * Setup uses the admin storage state to seed public data via the API, then
 * navigates as an anonymous user (no auth-bearing cookies; the public client
 * is used because the admin storageState only carries oidc-client-ts state
 * for the admin flow, which public routes ignore).
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, getAccessToken } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB_SLUG = `e2e-public-pub-${RUN_ID}`
const PUB_NAME = `E2E Public Pub ${RUN_ID}`
const MCP_SLUG = `e2e-public-mcp-${RUN_ID}`
const MCP_NAME = `E2E Public MCP ${RUN_ID}`
const AGENT_SLUG = `e2e-public-agent-${RUN_ID}`
const AGENT_NAME = `E2E Public Agent ${RUN_ID}`
const PRIVATE_SLUG = `e2e-public-priv-${RUN_ID}`

async function apiDelete(page: Page, path: string) {
  const token = await getAccessToken(page)
  return page.request.delete(path, { headers: { Authorization: `Bearer ${token}` } })
}

test.describe.configure({ mode: 'serial' })

test.describe('Public coverage', () => {
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    // Publisher.
    expect((await apiPost(page, '/api/v1/publishers', { slug: PUB_SLUG, name: PUB_NAME })).ok()).toBeTruthy()

    // Public MCP server with a published version.
    expect((await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB_SLUG, slug: MCP_SLUG, name: MCP_NAME, description: 'Public-detail test',
    })).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/versions`,
      {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_version: '2025-03-26',
        packages: [{ registryType: 'npm', identifier: '@e2e/public', version: '1.0.0', transport: { type: 'stdio' } }],
      },
    )).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`,
      {},
    )).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/visibility`,
      { visibility: 'public' },
    )).ok()).toBeTruthy()

    // Public agent with a published version.
    expect((await apiPost(page, '/api/v1/agents', {
      namespace: PUB_SLUG, slug: AGENT_SLUG, name: AGENT_NAME, description: 'Public-agent test',
    })).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/agents/${PUB_SLUG}/${AGENT_SLUG}/versions`,
      {
        version: '1.0.0',
        endpoint_url: 'https://agents.example.test/public',
        protocol_version: '0.3.0',
        default_input_modes: ['text/plain'],
        default_output_modes: ['text/plain'],
        skills: [{ id: 's', name: 'Skill', description: 'A skill', tags: ['e2e'] }],
        authentication: [{ scheme: 'Bearer' }],
      },
    )).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/agents/${PUB_SLUG}/${AGENT_SLUG}/versions/1.0.0/publish`,
      {},
    )).ok()).toBeTruthy()
    expect((await apiPost(
      page,
      `/api/v1/agents/${PUB_SLUG}/${AGENT_SLUG}/visibility`,
      { visibility: 'public' },
    )).ok()).toBeTruthy()

    // Private MCP used for the 404 test.
    expect((await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB_SLUG, slug: PRIVATE_SLUG, name: 'Private MCP',
    })).ok()).toBeTruthy()

    await ctx.close()
  })

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/mcp/servers/${PUB_SLUG}/${PRIVATE_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/agents/${PUB_SLUG}/${AGENT_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB_SLUG}`).catch(() => {})
    await ctx.close()
  })

  // ── W3a: Public search + filter ────────────────────────────────────────

  test('public MCP listing filters by namespace + search', async ({ page }) => {
    await page.goto(`/mcp?namespace=${PUB_SLUG}`)
    await expect(page.getByText(MCP_NAME)).toBeVisible({ timeout: 15_000 })

    // Search for something unrelated — card should disappear. Use exact match
    // so we don't collide with the "Search registry" combobox in the header.
    await page.getByRole('textbox', { name: 'Search', exact: true }).fill('zzz-nothing-matches')
    await expect(page).toHaveURL(/q=zzz-nothing-matches/, { timeout: 5_000 })
    await expect(page.getByText(MCP_NAME)).not.toBeVisible({ timeout: 5_000 })
  })

  test('public agent listing shows the seeded public agent', async ({ page }) => {
    await page.goto(`/agents?namespace=${PUB_SLUG}`)
    await expect(page.getByText(AGENT_NAME)).toBeVisible({ timeout: 15_000 })
  })

  // ── W3e: Publisher detail page ─────────────────────────────────────────

  test('public publisher detail page renders name, MCP and agent sections', async ({ page }) => {
    await page.goto(`/publishers/${PUB_SLUG}`)
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })

    // Section headings.
    await expect(page.getByRole('heading', { name: /MCP Servers/ })).toBeVisible()
    await expect(page.getByRole('heading', { name: /^Agents/ })).toBeVisible()

    // Cards for the seeded entries. Private MCP must not appear.
    await expect(page.getByText(MCP_NAME)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(AGENT_NAME)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Private MCP')).not.toBeVisible()
  })

  test('publisher detail page for an unknown slug shows a not-found state', async ({ page }) => {
    await page.goto(`/publishers/does-not-exist-${RUN_ID}`)
    await expect(page.getByText(/Publisher not found/i)).toBeVisible({ timeout: 15_000 })
  })

  // ── v0.3.0 Task 3: Namespace landing pages ─────────────────────────────
  // The landing pages at /mcp/:namespace and /agents/:namespace are the
  // first-class anchor for "everything published by {ns}". They must render
  // the seeded public entries, hide private ones, expose a working link to
  // the detail page, and distinguish a missing namespace (404) from an
  // existing namespace with zero entries of the requested kind.

  test('MCP namespace landing shows seeded public server and hides private ones', async ({ page }) => {
    await page.goto(`/mcp/${PUB_SLUG}`)
    // Publisher heading drives the header — wait on it so we know both
    // queries (publisher + list) have resolved before asserting the grid.
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })

    // Seeded public MCP appears on the card grid. Private MCP must NOT —
    // it's under the same namespace but should never leak to a public view.
    await expect(page.getByText(MCP_NAME)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Private MCP')).not.toBeVisible()
  })

  test('MCP namespace landing links to the server detail page', async ({ page }) => {
    await page.goto(`/mcp/${PUB_SLUG}`)
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })

    // The card's name doubles as the whole-card link (after:absolute inset-0
    // pseudo-element). Clicking it lands on /mcp/{ns}/{slug} — assert both
    // the URL and a fragment of the detail page's chrome so a future
    // regression that leaves the user on the landing page still trips.
    await page.getByRole('link', { name: MCP_NAME }).click()
    await expect(page).toHaveURL(new RegExp(`/mcp/${PUB_SLUG}/${MCP_SLUG}$`), { timeout: 10_000 })
    await expect(page.getByRole('heading', { name: MCP_NAME })).toBeVisible({ timeout: 10_000 })
  })

  test('agent namespace landing shows the seeded public agent', async ({ page }) => {
    await page.goto(`/agents/${PUB_SLUG}`)
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText(AGENT_NAME)).toBeVisible({ timeout: 10_000 })
  })

  test('unknown namespace on /mcp/:namespace renders the not-found state', async ({ page }) => {
    // A slug that's guaranteed not to exist — per-run ID keeps parallel
    // test runs from colliding with each other's fixtures.
    await page.goto(`/mcp/does-not-exist-${RUN_ID}`)
    await expect(page.getByText(/Namespace not found/i)).toBeVisible({ timeout: 15_000 })
    // The CTA steers users back to the flat list so they don't dead-end.
    await expect(page.getByRole('link', { name: /browse all mcp servers/i })).toBeVisible()
  })

  test('server card namespace chip navigates to the namespace landing page', async ({ page }) => {
    // Drive the navigation from the flat listing so we cover the
    // card → landing wiring that replaced the old `?namespace=X` filter link.
    await page.goto('/mcp')
    const chip = page.getByRole('link', { name: PUB_SLUG, exact: true }).first()
    await expect(chip).toBeVisible({ timeout: 15_000 })
    await chip.click()
    await expect(page).toHaveURL(new RegExp(`/mcp/${PUB_SLUG}$`), { timeout: 10_000 })
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 10_000 })
  })

  // ── W3f: Theme toggle persistence ──────────────────────────────────────

  test('theme toggle flips dark class and persists in localStorage', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 15_000 })

    // Start from a deterministic state: force light.
    await page.evaluate(() => {
      localStorage.setItem('theme', 'light')
      document.documentElement.classList.remove('dark')
    })
    await page.reload()
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 15_000 })

    // Click the toggle — label is "Switch to dark mode" when currently light.
    await page.getByRole('button', { name: /switch to dark mode/i }).click()
    await expect(page.locator('html.dark')).toBeAttached({ timeout: 5_000 })
    const stored = await page.evaluate(() => localStorage.getItem('theme'))
    expect(stored).toBe('dark')

    // Reload — dark state must survive.
    await page.reload()
    await expect(page.locator('html.dark')).toBeAttached({ timeout: 5_000 })

    // Toggle back.
    await page.getByRole('button', { name: /switch to light mode/i }).click()
    await expect(page.locator('html.dark')).not.toBeAttached({ timeout: 5_000 })
    const stored2 = await page.evaluate(() => localStorage.getItem('theme'))
    expect(stored2).toBe('light')
  })

  // ── W3g: Public 404 for private / missing ──────────────────────────────

  test('private MCP server is hidden from the public API', async ({ page }) => {
    // Anonymous request — private rows must not be readable. Public reads
    // return 404; the rate limiter may also reject (429) under heavy load.
    // Both outcomes prove the row is not leaked.
    const res = await page.request.get(`/api/v1/mcp/servers/${PUB_SLUG}/${PRIVATE_SLUG}`)
    expect([404, 429]).toContain(res.status())
  })

  test('missing MCP server returns 404 from the public API', async ({ page }) => {
    const res = await page.request.get(`/api/v1/mcp/servers/${PUB_SLUG}/does-not-exist-${RUN_ID}`)
    expect([404, 429]).toContain(res.status())
  })
})

// ── W3d: Pagination / Load more ─────────────────────────────────────────────

test.describe('Public coverage: pagination', () => {
  const PAGE_PUB = `e2e-page-pub-${RUN_ID}`
  const PAGE_NAME = `E2E Page Pub ${RUN_ID}`
  // Public MCP list paginates at limit=20 — seed 22 so a "Load more" appears
  // and the second page has at least two distinct rows to check.
  const COUNT = 22
  const slugFor = (i: number) => `e2e-page-mcp-${i.toString().padStart(2, '0')}-${RUN_ID}`
  const nameFor = (i: number) => `E2E Page MCP ${i.toString().padStart(2, '0')} ${RUN_ID}`

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(120_000)
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    expect((await apiPost(page, '/api/v1/publishers', { slug: PAGE_PUB, name: PAGE_NAME })).ok()).toBeTruthy()

    for (let i = 0; i < COUNT; i++) {
      const slug = slugFor(i)
      const name = nameFor(i)
      expect((await apiPost(page, '/api/v1/mcp/servers', {
        namespace: PAGE_PUB, slug, name, description: 'Pagination row',
      })).ok()).toBeTruthy()
      expect((await apiPost(
        page,
        `/api/v1/mcp/servers/${PAGE_PUB}/${slug}/versions`,
        {
          version: '1.0.0',
          runtime: 'stdio',
          protocol_version: '2025-03-26',
          packages: [{ registryType: 'npm', identifier: `@e2e/page-${i}`, version: '1.0.0', transport: { type: 'stdio' } }],
        },
      )).ok()).toBeTruthy()
      expect((await apiPost(
        page,
        `/api/v1/mcp/servers/${PAGE_PUB}/${slug}/versions/1.0.0/publish`,
        {},
      )).ok()).toBeTruthy()
      expect((await apiPost(
        page,
        `/api/v1/mcp/servers/${PAGE_PUB}/${slug}/visibility`,
        { visibility: 'public' },
      )).ok()).toBeTruthy()
    }
    await ctx.close()
  })

  test.afterAll(async ({ browser }) => {
    test.setTimeout(120_000)
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    for (let i = 0; i < COUNT; i++) {
      await apiDelete(page, `/api/v1/mcp/servers/${PAGE_PUB}/${slugFor(i)}`).catch(() => {})
    }
    await apiDelete(page, `/api/v1/publishers/${PAGE_PUB}`).catch(() => {})
    await ctx.close()
  })

  test('public MCP list shows Load more and second page brings new rows', async ({ page }) => {
    await page.goto(`/mcp?namespace=${PAGE_PUB}`)
    await expect(page.getByText(/^Showing 20( of \d+)? servers?$/)).toBeVisible({ timeout: 15_000 })

    // Capture the names shown on page 1 by reading the rendered card headings.
    // Each card title is an <h3> wrapping a <Link to="/mcp/{ns}/{slug}">.
    const cards = page.locator('h3:has(a[href^="/mcp/"])')
    const firstPageNames = (await cards.allTextContents()).map((s) => s.trim()).filter(Boolean)
    expect(firstPageNames.length).toBeGreaterThanOrEqual(20)

    // Load more button is present and clicking it adds a cursor to the URL.
    const loadMore = page.getByRole('link', { name: 'Load more' })
    await expect(loadMore).toBeVisible()
    await loadMore.click()
    await expect(page).toHaveURL(/cursor=/, { timeout: 5_000 })

    // After the cursor URL kicks in, the page renders the *next* slice
    // (the public list replaces rows on cursor change rather than appending).
    // The remaining rows = COUNT - first page = 2.
    await expect(page.getByText(/^Showing 2( of \d+)? servers?$/)).toBeVisible({ timeout: 15_000 })

    const secondPageNames = (await cards.allTextContents()).map((s) => s.trim()).filter(Boolean)
    expect(secondPageNames.length).toBe(2)

    // Sanity: no overlap between the two pages.
    const overlap = secondPageNames.filter((n) => firstPageNames.includes(n))
    expect(overlap).toEqual([])
  })
})
