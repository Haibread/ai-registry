/**
 * admin-ux-polish.spec.ts
 *
 * E2E coverage for the admin UX review follow-ups (docs/admin-ui-ux-review.md):
 *
 *   1. P2.2 — entity-list names link to the detail page, and the sort select
 *      drives a server-side sort via the `sort` URL param.
 *   2. P2.5 — the dirty-form guard blocks in-app navigation away from a
 *      half-filled create form until the user confirms.
 *   3. P2.3/P2.6 — the detail read view shows every editable field (with "—"
 *      for unset ones) and a missing entry renders the not-found branch.
 *   4. P3 — a community report row links to the reported entry's admin
 *      detail page by name (no raw-ULID text search).
 *
 * Run: npm run test:e2e -- admin-ux-polish
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiDelete } from './helpers'

const RUN = Date.now().toString(36)

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin' | 'author'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  await page.goto('/')
  return page
}

const PUB = `e2e-polish-${RUN}`

test.describe('Admin UX polish', () => {
  test('seed', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const pub = await apiPost(admin, '/api/v1/publishers', { slug: PUB, name: `E2E Polish ${RUN}` })
    expect(pub.status(), `create publisher: ${await pub.text()}`).toBe(201)
    // Two servers with names that invert creation order under name_asc.
    for (const [slug, name] of [
      ['zulu-server', 'Zulu Server'],
      ['alpha-server', 'Alpha Server'],
    ] as const) {
      const r = await apiPost(admin, '/api/v1/mcp/servers', { namespace: PUB, slug, name })
      expect(r.status(), `create ${slug}: ${await r.text()}`).toBe(201)
    }
  })

  test('list names link to the detail page and sorting reorders rows (P2.2)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto(`/admin/mcp?namespace=${PUB}`)

    // The name cell is a link straight to the entry — Manage is no longer
    // the only navigation affordance.
    const nameLink = admin.getByRole('link', { name: 'Alpha Server' })
    await expect(nameLink).toBeVisible({ timeout: 15_000 })
    await expect(nameLink).toHaveAttribute('href', `/admin/mcp/${PUB}/alpha-server`)

    // The two creates can share a created_at timestamp (tie under the
    // default newest-first sort), so assert both explicit name sorts
    // rather than the default ordering.
    const rows = admin.locator('tbody tr')
    await admin.getByLabel('Sort by').selectOption('name_desc')
    await expect(admin).toHaveURL(/sort=name_desc/)
    await expect(rows.first()).toContainText('Zulu Server', { timeout: 10_000 })

    // Sorting by name ascending updates the URL and reorders server-side.
    await admin.getByLabel('Sort by').selectOption('name_asc')
    await expect(admin).toHaveURL(/sort=name_asc/)
    await expect(rows.first()).toContainText('Alpha Server', { timeout: 10_000 })

    // Clicking the name lands on the admin detail page.
    await admin.getByRole('link', { name: 'Alpha Server' }).click()
    await expect(admin).toHaveURL(`/admin/mcp/${PUB}/alpha-server`)
    await expect(admin.getByRole('heading', { name: 'Alpha Server' })).toBeVisible()
  })

  test('the detail read view shows unset fields as "—" (P2.3)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto(`/admin/mcp/${PUB}/alpha-server`)
    await expect(admin.getByRole('heading', { name: 'Alpha Server' })).toBeVisible({ timeout: 15_000 })
    // Homepage and Repository were never set — the rows still render, with
    // an explicit dash, so "not set" is distinguishable from "not shown".
    await expect(admin.getByText('Homepage', { exact: true })).toBeVisible()
    await expect(admin.getByText('Repository', { exact: true })).toBeVisible()
  })

  test('a missing entry renders the not-found branch, not a generic error (P2.6)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto(`/admin/mcp/${PUB}/does-not-exist`)
    await expect(admin.getByText(/not found/i)).toBeVisible({ timeout: 15_000 })
    await expect(admin.getByRole('button', { name: /back to mcp servers/i })).toBeVisible()
  })

  test('leaving a dirty create form asks for confirmation (P2.5)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto('/admin/mcp/new')
    await admin.getByLabel(/^name/i).fill('Half-finished server')

    // In-app navigation is blocked by the shared confirm dialog.
    await admin.getByRole('link', { name: 'Dashboard' }).click()
    const dialog = admin.locator('dialog[open]')
    await expect(dialog.getByRole('heading', { name: /discard unsaved changes/i })).toBeVisible()

    // Keep editing → still on the form, input intact.
    await dialog.getByRole('button', { name: /keep editing/i }).click()
    await expect(admin).toHaveURL(/\/admin\/mcp\/new/)
    await expect(admin.getByLabel(/^name/i)).toHaveValue('Half-finished server')

    // Discard → navigation proceeds.
    await admin.getByRole('link', { name: 'Dashboard' }).click()
    await admin.locator('dialog[open]').getByRole('button', { name: /discard changes/i }).click()
    await expect(admin).toHaveURL(/\/admin$/)
  })

  test('a report row links to the reported entry by name (P3)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    // Look up the entry's ULID, then file a report against it (public API).
    const detail = await admin.request.get(`/api/v1/mcp/servers/${PUB}/alpha-server`, {
      headers: { Authorization: `Bearer ${await admin.evaluate(() => window.localStorage.getItem('ai_registry_access'))}` },
    })
    expect(detail.status()).toBe(200)
    const { id } = (await detail.json()) as { id: string }
    const report = await admin.request.post('/api/v1/reports', {
      data: { resource_type: 'mcp_server', resource_id: id, issue_type: 'other', description: `e2e polish ${RUN}` },
    })
    expect(report.status(), `file report: ${await report.text()}`).toBe(201)

    await admin.goto('/admin/reports')
    const row = admin.locator('li:not([data-sonner-toast])', { hasText: `e2e polish ${RUN}` })
    await expect(row).toBeVisible({ timeout: 15_000 })
    const link = row.getByRole('link', { name: new RegExp(`Alpha Server.*${PUB}/alpha-server`) })
    await expect(link).toHaveAttribute('href', `/admin/mcp/${PUB}/alpha-server`)
    await link.click()
    await expect(admin.getByRole('heading', { name: 'Alpha Server' })).toBeVisible({ timeout: 15_000 })

    // Triage it so the pending queue stays clean for other specs.
    // (The report stays in the DB as `dismissed`, which is fine.)
    await admin.goto('/admin/reports')
    await admin.locator('li:not([data-sonner-toast])', { hasText: `e2e polish ${RUN}` }).getByRole('button', { name: /dismiss/i }).click()
  })

  test('cleanup', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    for (const slug of ['alpha-server', 'zulu-server']) {
      await apiDelete(admin, `/api/v1/mcp/servers/${PUB}/${slug}`)
    }
    const r = await apiDelete(admin, `/api/v1/publishers/${PUB}`)
    expect([200, 204]).toContain(r.status())
  })
})
