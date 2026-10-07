/**
 * detail.spec.ts
 *
 * End-to-end tests for the public MCP server detail page.
 *
 * The admin CRUD suite in admin.spec.ts creates entries but never navigates
 * into their *public* detail pages, and public.spec.ts only covers listings.
 * These tests close that gap by asserting the v0.2 Connection card, tab
 * navigation, and raw JSON all render against a real backend.
 *
 * Strategy:
 *   - Seed a publisher and an MCP server (with a remote package so the
 *     Connection & Runtime hero row renders) via the admin API using the
 *     shared storageState.
 *   - Publish and make the entry public.
 *   - Navigate as a page user and assert detail-page content.
 *   - Tear everything down in afterAll so the run is idempotent.
 *
 * Uses the admin storageState (injected via playwright.config.ts) so the tests
 * can authenticate API calls for seeding and teardown. The detail pages being
 * tested are public reads — authentication does not alter their content.
 */

import { test, expect } from '@playwright/test'
import { apiCleanup, apiPost, confirmDialog } from './helpers'

// Unique suffix to avoid collisions across runs.
const RUN_ID = Date.now().toString(36)
const PUBLISHER_SLUG = `e2e-detail-pub-${RUN_ID}`
const PUBLISHER_NAME = `E2E Detail Publisher ${RUN_ID}`
const MCP_SLUG = `e2e-detail-mcp-${RUN_ID}`
const MCP_NAME = `E2E Detail MCP ${RUN_ID}`

// Tests seed state once and read it; keep them serial so a test can assume the
// previous step completed successfully.
test.describe.configure({ mode: 'serial' })

test.describe('Public detail pages', () => {
  test.beforeAll(async ({ browser }) => {
    const context = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await context.newPage()

    // Navigate first so localStorage is accessible for apiPost.
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    // ── Publisher ────────────────────────────────────────────────────────
    const pubRes = await apiPost(page, '/api/v1/publishers', {
      slug: PUBLISHER_SLUG,
      name: PUBLISHER_NAME,
    })
    if (!pubRes.ok()) {
      throw new Error(`seed publisher failed: ${pubRes.status()} ${await pubRes.text()}`)
    }

    // ── MCP server with a remote package ─────────────────────────────────
    const mcpRes = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUBLISHER_SLUG,
      slug: MCP_SLUG,
      name: MCP_NAME,
      description: 'An E2E detail page MCP server.',
    })
    if (!mcpRes.ok()) {
      throw new Error(`seed mcp server failed: ${mcpRes.status()} ${await mcpRes.text()}`)
    }

    const mcpVerRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`,
      {
        version: '1.0.0',
        runtime: 'sse',
        protocol_versions: ['2025-03-26'],
        packages: [
          {
            registryType: 'npm',
            identifier: '@e2e/detail-mcp',
            version: '1.0.0',
            transport: { type: 'sse', url: 'https://mcp.example.test/e2e-detail/sse' },
          },
        ],
      },
    )
    if (!mcpVerRes.ok()) {
      throw new Error(`seed mcp version failed: ${mcpVerRes.status()} ${await mcpVerRes.text()}`)
    }

    const mcpPubRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`,
      {},
    )
    if (!mcpPubRes.ok()) {
      throw new Error(`publish mcp version failed: ${mcpPubRes.status()} ${await mcpPubRes.text()}`)
    }

    const mcpVisRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/visibility`,
      { visibility: 'public' },
    )
    if (!mcpVisRes.ok()) {
      throw new Error(`make mcp public failed: ${mcpVisRes.status()} ${await mcpVisRes.text()}`)
    }

    // Sanity check — the public API can read it back without a token.
    const anonMcp = await page.request.get(
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}`,
    )
    if (!anonMcp.ok()) {
      throw new Error(`anon read mcp failed: ${anonMcp.status()}`)
    }

    await context.close()
  })

  test.afterAll(async ({ browser }) => {
    const context = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await context.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    await apiCleanup(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await apiCleanup(page, `/api/v1/publishers/${PUBLISHER_SLUG}`)

    await context.close()
  })

  // ── MCP detail page ───────────────────────────────────────────────────

  test('MCP detail page renders the name and the Quick connect card', async ({ page }) => {
    await page.goto(`/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByRole('heading', { name: MCP_NAME })).toBeVisible({ timeout: 15_000 })

    // A remote server's endpoint URL is copyable from the side column.
    const connect = page.getByRole('region', { name: 'Quick connect' })
    await expect(connect.getByText('https://mcp.example.test/e2e-detail/sse')).toBeVisible()
    await expect(connect.getByRole('button', { name: /copy endpoint url/i })).toBeVisible()

    // The runtime is labelled as the transport for remote servers.
    await expect(page.getByRole('region', { name: 'Details' }).getByText('Transport')).toBeVisible()
  })

  test('MCP detail page has Overview/Usage/Versions tabs and the raw JSON', async ({ page }) => {
    await page.goto(`/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByRole('heading', { name: MCP_NAME })).toBeVisible({ timeout: 15_000 })

    await expect(page.getByRole('tab', { name: /overview/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /usage/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /versions/i })).toBeVisible()

    // The raw server document folds at the bottom of the Overview tab. The
    // slug appears elsewhere on the page (sticky header, dialogs), so scope
    // the assertion to the active panel's <pre> element.
    await page.getByRole('button', { name: /raw api response/i }).click()
    const jsonPre = page.locator('[role="tabpanel"][data-state="active"] pre')
    await expect(jsonPre).toBeVisible({ timeout: 10_000 })
    await expect(jsonPre).toContainText(MCP_SLUG)
    await expect(jsonPre).toContainText('@e2e/detail-mcp')

    // Usage tab shows the package identifier in the config generator.
    await page.getByRole('tab', { name: /usage/i }).click()
    await expect(page.getByText(/@e2e\/detail-mcp@1\.0\.0/).first()).toBeVisible({ timeout: 10_000 })
  })

  test('publisher Markdown replaces the generated Usage tab, then resets', async ({ page }) => {
    await page.goto(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await page.getByRole('button', { name: 'Customize' }).click()
    await page
      .getByRole('textbox', { name: 'Usage Markdown' })
      .fill('## Before you start\n\nRequires v{{version}}.\n\n- Bring a token\n- Upload over HTTP')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(page.getByText('Custom', { exact: true })).toBeVisible()

    await page.goto(`/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}#usage`)
    await expect(page.getByRole('heading', { name: 'Before you start' })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Requires v1.0.0.')).toBeVisible()
    await expect(page.getByText('Generated configuration')).toBeVisible()

    const fontSize = (el: Element) => parseFloat(getComputedStyle(el).fontSize)
    const headingSize = await page.getByRole('heading', { name: 'Before you start' }).evaluate(fontSize)
    const bodySize = await page.getByText('Requires v1.0.0.').evaluate(fontSize)
    expect(headingSize).toBeGreaterThan(bodySize)
    const list = page.getByRole('list').filter({ hasText: 'Bring a token' })
    await expect(list).toHaveCSS('list-style-type', 'disc')

    await page.goto(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await page.getByRole('button', { name: 'Reset to generated' }).click()
    await confirmDialog(page, 'Reset')
    await expect(page.getByText('Generated', { exact: true })).toBeVisible()
  })
})
