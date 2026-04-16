/**
 * activity.spec.ts
 *
 * Smoke tests for the per-entry activity feed + admin audit page.
 *
 * Covers:
 *  1. Seed a publisher + MCP server + published version via the admin API so
 *     the audit log has real events to show.
 *  2. Admin /audit page renders the events with full actor identity.
 *  3. Admin filter by resource_type narrows the list.
 *  4. The PUBLIC MCP detail page renders an "Activity" feed with the seeded
 *     events.
 *  5. Privacy scrub (critical): the public feed MUST NOT leak actor subject,
 *     actor email, client IP, or internal metadata, even when those are
 *     present in the raw audit row. The scrub happens server-side in
 *     /api/v1/mcp/servers/{ns}/{slug}/activity.
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB_SLUG = `act-pub-${RUN_ID}`
const MCP_SLUG = `act-mcp-${RUN_ID}`
const MCP_NAME = `Activity MCP ${RUN_ID}`

async function goTo(page: Page, path: string) {
  await page.goto(path)
  await page.waitForLoadState('domcontentloaded')
}

test.describe('Activity feed + admin audit', () => {
  test.beforeAll(async ({ browser }) => {
    // Seed data via the admin API so both the public feed and admin audit
    // surface have something real to render.
    const ctx = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await ctx.newPage()
    await goTo(page, '/admin')
    // Wait for the admin shell so localStorage is populated with the token.
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    const pub = await apiPost(page, '/api/v1/publishers', {
      slug: PUB_SLUG,
      name: `Activity Publisher ${RUN_ID}`,
    })
    expect(pub.ok()).toBeTruthy()

    const mcp = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB_SLUG,
      slug: MCP_SLUG,
      name: MCP_NAME,
      description: 'Seeded by activity.spec.ts',
    })
    expect(mcp.ok()).toBeTruthy()

    // Make it public so the public detail page renders it.
    const vis = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/visibility`,
      { visibility: 'public' },
    )
    expect(vis.ok()).toBeTruthy()

    // Create + publish a version to produce a version.published audit event.
    const verRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/versions`,
      {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_version: '2025-03-26',
        packages: [
          {
            registryType: 'npm',
            identifier: '@activity/test',
            version: '1.0.0',
            transport: { type: 'stdio' },
          },
        ],
      },
    )
    expect(verRes.ok()).toBeTruthy()

    const pubRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`,
      {},
    )
    expect(pubRes.ok()).toBeTruthy()

    await ctx.close()
  })

  test('admin /audit page renders events with actor identity', async ({ page }) => {
    await goTo(page, '/admin/audit')
    await expect(page.getByRole('heading', { name: /activity/i })).toBeVisible({
      timeout: 15_000,
    })

    // The seeded MCP mutation should surface as a row whose drill-down link
    // points at the admin MCP detail page.
    const drillLink = page.getByRole('link', {
      name: new RegExp(`${PUB_SLUG}/${MCP_SLUG}`),
    })
    await expect(drillLink.first()).toBeVisible({ timeout: 10_000 })
    await expect(drillLink.first()).toHaveAttribute(
      'href',
      `/admin/mcp/${PUB_SLUG}/${MCP_SLUG}`,
    )

    // Actor identity columns are admin-only. An email and a subject should
    // appear somewhere on the page (the seed ran as the admin session user).
    await expect(page.getByText(/actor:/i).first()).toBeVisible()
    await expect(page.getByText(/subject:/i).first()).toBeVisible()
  })

  test('admin audit filter by resource type narrows the list', async ({ page }) => {
    await goTo(page, '/admin/audit')
    await expect(page.getByRole('heading', { name: /activity/i })).toBeVisible({
      timeout: 15_000,
    })

    // Open the resource-type Select and pick "Agents" — our seeded MCP rows
    // should disappear from view.
    await page.getByLabel('Resource type').click()
    await page.getByRole('option', { name: /agents/i }).click()
    // Wait for the filtered refetch: the seeded MCP slug must not appear.
    await expect(
      page.getByRole('link', {
        name: new RegExp(`${PUB_SLUG}/${MCP_SLUG}`),
      }),
    ).toHaveCount(0, { timeout: 10_000 })
  })

  test('public MCP detail page renders the activity feed', async ({ page }) => {
    await goTo(page, `/mcp/${PUB_SLUG}/${MCP_SLUG}`)
    // The detail header renders the name; wait for it so the page is hydrated.
    await expect(page.getByText(MCP_NAME).first()).toBeVisible({ timeout: 15_000 })

    // The public feed renders at least one of the seeded lifecycle events.
    // mcp_server.created is always in the whitelist.
    await expect(
      page.getByText(/created|published/i).first(),
    ).toBeVisible({ timeout: 10_000 })
  })

  test('public activity feed does not leak admin-only fields', async ({ page }) => {
    const res = await page.request.get(
      `/api/v1/mcp/servers/${PUB_SLUG}/${MCP_SLUG}/activity`,
    )
    expect(res.ok()).toBeTruthy()
    const body = (await res.json()) as { items: unknown[] }
    expect(Array.isArray(body.items)).toBe(true)
    expect(body.items.length).toBeGreaterThan(0)

    // Serialize the whole response and assert none of the scrubbed keys / PII
    // markers ever appear in the wire payload.
    const serialized = JSON.stringify(body)
    expect(serialized).not.toContain('actor_subject')
    expect(serialized).not.toContain('actor_email')
    expect(serialized).not.toContain('client_ip')
    expect(serialized).not.toContain('user_agent')
    expect(serialized).not.toContain('internal_note')

    // Each event MUST expose actor_role but NEVER a Keycloak UUID or email.
    for (const raw of body.items) {
      const e = raw as Record<string, unknown>
      expect(typeof e.actor_role).toBe('string')
      expect(e.actor_role).toBe('admin')
      expect(e).not.toHaveProperty('actor_subject')
      expect(e).not.toHaveProperty('actor_email')
    }
  })
})
