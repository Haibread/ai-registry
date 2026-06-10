/**
 * request-public.spec.ts
 *
 * E2E coverage for the author-side release intent ("create and ask for
 * publish"): a version submission can request that the (private) entry be
 * made public when approved. The queue shows the intent to the reviewer,
 * approval flips visibility atomically, and a rejection leaves the entry
 * private.
 *
 * Run: npm run test:e2e -- --project=request-public
 */

import { test, expect } from '@playwright/test'
import { apiPost, apiGet, apiDelete, confirmDialog } from './helpers'

const RUN = Date.now().toString(36)
const PUB = `e2e-reqpub-${RUN}`
const APPROVED = 'goes-public'
const REJECTED = 'stays-private'

test.use({ storageState: 'e2e/.auth/admin.json' })
test.describe.configure({ mode: 'serial' })

test.describe('Request public release on approval', () => {
  test('seed', async ({ page }) => {
    await page.goto('/')
    const pub = await apiPost(page, '/api/v1/publishers', { slug: PUB, name: `E2E ReqPub ${RUN}` })
    expect(pub.status(), `create publisher: ${await pub.text()}`).toBe(201)
    for (const slug of [APPROVED, REJECTED]) {
      const srv = await apiPost(page, '/api/v1/mcp/servers', { namespace: PUB, slug, name: slug })
      expect(srv.status(), `create ${slug}: ${await srv.text()}`).toBe(201)
      const ver = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${slug}/versions`, {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_version: '2024-11-05',
        packages: [
          { registryType: 'npm', identifier: `@e2e/${slug}`, version: '1.0.0', transport: { type: 'stdio' } },
        ],
      })
      expect(ver.status(), `create version on ${slug}: ${await ver.text()}`).toBe(201)
    }
  })

  test('the submit dialog carries the request and the row flags it', async ({ page }) => {
    await page.goto(`/admin/mcp/${PUB}/${APPROVED}`)
    const row = page.locator('tr', { hasText: 'v1.0.0' })
    await row.getByRole('button', { name: /^submit$/i }).click()

    // The dialog offers the author-side release intent (entry is private).
    await page.getByLabel(/also request making this entry public/i).check()
    await confirmDialog(page, /^submit for review$/i)

    // The pending row announces the intent back to the author.
    await expect(row.getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })
    await expect(row.getByText(/public on approval/i)).toBeVisible()
  })

  test('the queue shows the intent and approval makes the entry public', async ({ page }) => {
    await page.goto('/admin/review')
    const queueRow = page.locator('li', { hasText: `${PUB}/${APPROVED}` })
    await expect(queueRow).toBeVisible({ timeout: 15_000 })
    await expect(queueRow.getByText(/public on approval/i)).toBeVisible()

    await queueRow.getByRole('button', { name: /^approve$/i }).click()
    // The confirm spells out the bigger blast radius before publishing.
    await expect(page.getByText(/makes the entry public/i)).toBeVisible()
    await confirmDialog(page, /approve & publish/i)
    await expect(queueRow).toBeHidden({ timeout: 10_000 })

    // The entry is now publicly readable — no bearer token at all.
    const anon = await page.request.get(`/api/v1/mcp/servers/${PUB}/${APPROVED}`)
    expect(anon.status(), 'anonymous read of the now-public entry').toBe(200)
    expect((await anon.json()).visibility).toBe('public')
  })

  test('a rejected request leaves the entry private', async ({ page }) => {
    // Submit the second entry with the same request, via the API this time.
    const sub = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${REJECTED}/versions/1.0.0/submit`, {
      request_public: true,
    })
    expect(sub.status(), `submit: ${await sub.text()}`).toBe(204)

    await page.goto('/admin/review')
    const queueRow = page.locator('li', { hasText: `${PUB}/${REJECTED}` })
    await expect(queueRow).toBeVisible({ timeout: 15_000 })
    await queueRow.getByRole('button', { name: /^reject$/i }).click()
    await queueRow.getByPlaceholder(/reason/i).fill('not ready for the public registry')
    await queueRow.getByRole('button', { name: /confirm reject/i }).click()
    await expect(queueRow).toBeHidden({ timeout: 10_000 })

    // Still private: an anonymous read does not see it.
    const anon = await page.request.get(`/api/v1/mcp/servers/${PUB}/${REJECTED}`)
    expect(anon.status(), 'anonymous read of the still-private entry').toBe(404)
    const admin = await apiGet(page, `/api/v1/mcp/servers/${PUB}/${REJECTED}`)
    expect((await admin.json()).visibility).toBe('private')
  })

  test('cleanup', async ({ page }) => {
    await page.goto('/')
    for (const slug of [APPROVED, REJECTED]) {
      await apiDelete(page, `/api/v1/mcp/servers/${PUB}/${slug}`)
    }
    const r = await apiDelete(page, `/api/v1/publishers/${PUB}`)
    expect([200, 204]).toContain(r.status())
  })
})
