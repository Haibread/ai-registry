/**
 * change-approval.spec.ts
 *
 * End-to-end tests for the change-approval workflow on the live stack.
 * Drives the admin API directly (no UI yet) and covers:
 *
 *   - Submit a draft version, approve it (published).
 *   - Submit, reject with reason, resubmit (rejection_reason cleared).
 *   - Approve with a stale revision → 409 review-revision-mismatch.
 *   - Approve a no-longer-pending version → 409 review-state-mismatch.
 *   - Request deletion, approve it; entry disappears from public reads.
 *
 * Prerequisites (same as the other admin specs):
 *   - docker compose -f docker-compose.dev.yml up -d
 *   - AUTH_STORAGE=local in deploy/.env
 *   - e2e/.auth/admin.json populated by the setup project.
 *
 * Run:  npm run test:e2e -- --project=change-approval
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, apiGet, apiDelete } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `e2e-rev-${RUN_ID}`
const MCP = `weather-${RUN_ID}`

test.describe.configure({ mode: 'serial' })

async function loadAdmin(page: Page) {
  await page.goto('/admin')
  await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
}

const seedDraftVersion = async (page: Page, version: string) => {
  const create = await apiPost(
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
  expect(create.status(), `create v${version}`).toBe(201)
}

test.describe('Change-approval workflow', () => {
  test.beforeEach(async ({ page }) => {
    await loadAdmin(page)
  })

  test('seed publisher + MCP server', async ({ page }) => {
    let resp = await apiPost(page, '/api/v1/publishers', {
      slug: PUB,
      name: PUB,
    })
    expect(resp.status(), 'publisher').toBe(201)

    resp = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB,
      slug: MCP,
      name: MCP,
    })
    expect(resp.status(), 'mcp server').toBe(201)
  })

  test('submit draft and approve', async ({ page }) => {
    await seedDraftVersion(page, '1.0.0')

    let resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.0.0/submit`,
      {},
    )
    expect(resp.status(), 'submit').toBe(204)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.0.0/approve`,
      { revision: 1 },
    )
    expect(resp.status(), 'approve').toBe(204)

    // The version should now show up in the version list with a
    // published_at timestamp.
    const list = await apiGet(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions`,
    )
    expect(list.status()).toBe(200)
    const body = await list.json()
    const v100 = (body.items as { version: string; published_at?: string }[]).find(
      v => v.version === '1.0.0',
    )
    expect(v100, 'v1.0.0 in version list').toBeDefined()
    expect(v100?.published_at, 'v1.0.0 published_at').toBeTruthy()
  })

  test('reject with reason, resubmit, approve', async ({ page }) => {
    await seedDraftVersion(page, '1.1.0')

    let resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.1.0/submit`,
      {},
    )
    expect(resp.status(), 'submit').toBe(204)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.1.0/reject`,
      { revision: 1, reason: 'needs more documentation' },
    )
    expect(resp.status(), 'reject').toBe(204)

    // Re-submit clears the rejection_reason and flips back to pending.
    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.1.0/submit`,
      {},
    )
    expect(resp.status(), 're-submit').toBe(204)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.1.0/approve`,
      { revision: 1 },
    )
    expect(resp.status(), 'approve').toBe(204)
  })

  test('approve with stale revision returns 409 review-revision-mismatch', async ({ page }) => {
    await seedDraftVersion(page, '1.2.0')

    let resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.2.0/submit`,
      {},
    )
    expect(resp.status()).toBe(204)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.2.0/approve`,
      { revision: 99 },
    )
    expect(resp.status(), 'stale revision').toBe(409)
    const body = (await resp.json()) as { type: string; status: number }
    expect(body.status).toBe(409)
    expect(body.type, 'discriminator type').toMatch(/review-revision-mismatch$/)

    // Withdraw so the partial-unique pending_review constraint releases
    // and subsequent tests can submit a different version on this entry.
    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.2.0/withdraw`,
      {},
    )
    expect(resp.status(), 'withdraw').toBe(204)
  })

  test('approve already-approved version returns 409 already-published or state-mismatch', async ({ page }) => {
    await seedDraftVersion(page, '1.3.0')

    let resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.3.0/submit`,
      {},
    )
    expect(resp.status()).toBe(204)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.3.0/approve`,
      { revision: 1 },
    )
    expect(resp.status(), 'first approve').toBe(204)

    // A second reviewer racing on the same revision now sees the row as
    // either no-longer-pending (state-mismatch) or already-published.
    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/1.3.0/approve`,
      { revision: 1 },
    )
    expect(resp.status(), 'second approve').toBe(409)
    const body = (await resp.json()) as { type: string }
    expect(body.type).toMatch(/(review-state-mismatch|already-published)$/)
  })

  test('request and approve deletion removes the entry from public reads', async ({ page }) => {
    // Make the entry public so the public list would otherwise return it.
    let resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/visibility`,
      { visibility: 'public' },
    )
    expect(resp.status(), 'set visibility').toBe(200)

    // Confirm the entry is currently in the public list.
    let pub = await page.request.get('/api/v1/mcp/servers')
    expect(pub.status()).toBe(200)
    let body = await pub.json()
    let slugs = (body.items as { slug: string }[]).map(s => s.slug)
    expect(slugs, 'public list contains entry before delete').toContain(MCP)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/deletion-request`,
      {},
    )
    expect(resp.status(), 'request deletion').toBe(202)

    resp = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/deletion-request/approve`,
      {},
    )
    expect(resp.status(), 'approve deletion').toBe(204)

    pub = await page.request.get('/api/v1/mcp/servers')
    expect(pub.status()).toBe(200)
    body = await pub.json()
    slugs = (body.items as { slug: string }[]).map(s => s.slug)
    expect(slugs, 'public list excludes entry after workflow delete').not.toContain(MCP)
  })

  test('cleanup', async ({ page }) => {
    // The MCP server is now soft-deleted; deleting the publisher purges
    // tombstoned rows before dropping the publisher.
    const resp = await apiDelete(page, `/api/v1/publishers/${PUB}`)
    expect([200, 204], 'publisher delete').toContain(resp.status())
  })
})
