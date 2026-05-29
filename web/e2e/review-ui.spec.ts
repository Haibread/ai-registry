/**
 * review-ui.spec.ts
 *
 * End-to-end tests for the change-approval admin UI: review queue
 * page, version section workflow controls, and the request-deletion
 * button. Drives the actual browser DOM (unlike change-approval.spec
 * which goes through the API directly), so it catches handler/UI
 * contract drift the unit tests with mocked clients can't.
 *
 * Prerequisites: same as the other admin specs (docker-compose dev
 * stack with AUTH_STORAGE=local).
 *
 * Run:  npm run test:e2e -- --project=review-ui
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, apiDelete } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `e2e-review-ui-${RUN_ID}`
const MCP = `weather-${RUN_ID}`
const VER1 = '1.0.0'
const VER2 = '1.1.0'

test.describe.configure({ mode: 'serial' })

async function loadAdmin(page: Page) {
  await page.goto('/admin')
  await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
}

const seedDraftVersion = async (page: Page, version: string) => {
  const r = await apiPost(
    page,
    `/api/v1/mcp/servers/${PUB}/${MCP}/versions`,
    {
      version,
      runtime: 'stdio',
      packages: [
        {
          registryType: 'npm',
          identifier: '@scope/pkg',
          version: '1.0.0',
          transport: { type: 'stdio' },
        },
      ],
      capabilities: { tools: [] },
      protocol_version: '2024-11-05',
    },
  )
  expect(r.status(), `seed v${version}`).toBe(201)
}

test.describe('Admin review UI', () => {
  test.beforeEach(async ({ page }) => {
    await loadAdmin(page)
  })

  test('seed publisher + MCP server + draft', async ({ page }) => {
    let r = await apiPost(page, '/api/v1/publishers', { slug: PUB, name: PUB })
    expect(r.status(), 'publisher').toBe(201)
    r = await apiPost(page, '/api/v1/mcp/servers', { namespace: PUB, slug: MCP, name: MCP })
    expect(r.status(), 'mcp server').toBe(201)
    await seedDraftVersion(page, VER1)
  })

  test('Submit on the entry detail page moves the version to pending', async ({ page }) => {
    await page.goto(`/admin/mcp/${PUB}/${MCP}`)
    await expect(page.getByRole('heading', { name: /versions/i })).toBeVisible({ timeout: 15_000 })
    // Versions table should have a row for v1.0.0 with a Submit button.
    const row = page.locator('tr', { hasText: VER1 })
    await expect(row).toBeVisible()
    await row.getByRole('button', { name: /^submit$/i }).click()

    // After submit, the row's state badge flips to "pending review" and
    // the action button becomes Withdraw.
    await expect(row.getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })
    await expect(row.getByRole('button', { name: /^withdraw$/i })).toBeVisible()
  })

  test('Review queue page shows the pending version', async ({ page }) => {
    await page.goto('/admin/review')
    await expect(page.getByRole('heading', { name: /review queue/i })).toBeVisible({
      timeout: 15_000,
    })
    // Find the row by entry slug.
    await expect(page.getByText(`${PUB}/${MCP}`)).toBeVisible()
    // Version + revision badges from the queue.
    await expect(page.getByText(`v${VER1}`)).toBeVisible()
    await expect(page.getByText(/rev 1/i)).toBeVisible()
  })

  test('Reject from queue returns the version to draft (rejected) with the reason', async ({ page }) => {
    await page.goto('/admin/review')
    // Scope to the queue's version row; the trailing `.filter` excludes the
    // sonner success toast, which also contains the entry name (matches the
    // MCP-deletion row pattern below).
    const row = page.locator('li', { hasText: `${PUB}/${MCP}` }).filter({ hasText: /MCP version/i })
    await expect(row).toBeVisible()
    await row.getByRole('button', { name: /^reject$/i }).click()

    await row.getByPlaceholder(/reason/i).fill('needs more docs')
    await row.getByRole('button', { name: /confirm reject/i }).click()

    // Once rejected, the row leaves the queue (the queue only lists
    // pending_review items).
    await expect(row).toBeHidden({ timeout: 10_000 })

    // Back on the entry page, the row is rejected and shows the reason
    // we just entered. The action becomes Resubmit.
    await page.goto(`/admin/mcp/${PUB}/${MCP}`)
    const detailRow = page.locator('tr', { hasText: VER1 })
    await expect(detailRow.getByText(/rejected/i)).toBeVisible({ timeout: 10_000 })
    await expect(detailRow.getByText(/needs more docs/i)).toBeVisible()
    await expect(detailRow.getByRole('button', { name: /^resubmit$/i })).toBeVisible()
  })

  test('Approve from queue publishes the version', async ({ page }) => {
    // Resubmit first so the row reappears in the queue.
    await page.goto(`/admin/mcp/${PUB}/${MCP}`)
    const detailRow = page.locator('tr', { hasText: VER1 })
    await detailRow.getByRole('button', { name: /^resubmit$/i }).click()
    await expect(detailRow.getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })

    await page.goto('/admin/review')
    const queueRow = page.locator('li', { hasText: `${PUB}/${MCP}` }).filter({ hasText: /MCP version/i })
    await expect(queueRow).toBeVisible()
    await queueRow.getByRole('button', { name: /^approve$/i }).click()

    // Row leaves the queue; on the entry page the version is now
    // published (no review-state badge, no action buttons).
    await expect(queueRow).toBeHidden({ timeout: 10_000 })
    await page.goto(`/admin/mcp/${PUB}/${MCP}`)
    await expect(detailRow.getByText(/published/i)).toBeVisible({ timeout: 10_000 })
    await expect(detailRow.getByRole('button', { name: /submit|withdraw|resubmit/i })).toHaveCount(0)
  })

  test('Workspaces section on publisher detail can set a group binding', async ({ page }) => {
    await page.goto(`/admin/publishers/${PUB}`)
    // The default workspace should already exist (lazy-created on the
    // first MCP create earlier in this run).
    await expect(page.getByRole('heading', { name: /workspaces/i })).toBeVisible({
      timeout: 15_000,
    })
    // Click Edit on the default row.
    const defaultRow = page.locator('tr', { hasText: 'default' })
    await expect(defaultRow).toBeVisible()
    await defaultRow.getByRole('button', { name: /^edit$/i }).click()

    // The edit form's Keycloak group field defaults to the current
    // group_name (empty here) — fill it.
    await page.getByLabel(/keycloak group/i).fill('e2e-team-grp')
    await page.getByRole('button', { name: /save changes/i }).click()

    // Row re-renders with the new badge. Scope to the row because the
    // success toast also contains the group name and would collide with
    // a page-wide getByText match.
    await expect(defaultRow.getByText('e2e-team-grp')).toBeVisible({ timeout: 10_000 })

    // Clear it back to admin-only via another edit.
    await defaultRow.getByRole('button', { name: /^edit$/i }).click()
    const groupField = page.getByLabel(/keycloak group/i)
    await groupField.fill('')
    await page.getByRole('button', { name: /save changes/i }).click()
    await expect(defaultRow.getByText(/admin-only/i)).toBeVisible({ timeout: 10_000 })
  })

  test('Request deletion button on the entry submits a deletion review', async ({ page }) => {
    await page.goto(`/admin/mcp/${PUB}/${MCP}`)
    page.once('dialog', (d) => d.accept())
    await page.getByRole('button', { name: /request deletion \(review\)/i }).click()
    // Button flips to Pending review, success message renders.
    await expect(page.getByRole('button', { name: /pending review/i })).toBeDisabled({
      timeout: 10_000,
    })
    await expect(page.getByText(/a reviewer will approve or reject/i)).toBeVisible()

    // The review queue lists the deletion as a separate row.
    await page.goto('/admin/review')
    const delRow = page.locator('li', { hasText: `${PUB}/${MCP}` }).filter({ hasText: /MCP deletion/i })
    await expect(delRow).toBeVisible()

    // Approve the deletion.
    await delRow.getByRole('button', { name: /^approve$/i }).click()
    await expect(delRow).toBeHidden({ timeout: 10_000 })
  })

  test('cleanup', async ({ page }) => {
    // Publisher delete cascades through the (now-soft-deleted) MCP server.
    const r = await apiDelete(page, `/api/v1/publishers/${PUB}`)
    expect([200, 204]).toContain(r.status())
  })
})
