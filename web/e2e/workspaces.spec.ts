/**
 * workspaces.spec.ts
 *
 * End-to-end tests for the workspace API surface introduced in ADR 0001
 * (Phase 7.1) and ADR 0002 (Phase 7.2). Workspaces have no UI yet, so
 * these tests drive the live stack via the admin API and assert the full
 * loop: publisher -> workspace -> resource -> group binding -> auth.
 *
 * Prerequisites (same as the other admin specs):
 *   - docker compose -f docker-compose.dev.yml up -d
 *   - Realm imported, admin user provisioned, e2e/.auth/admin.json
 *     populated by the setup project.
 *
 * Run:  npm run test:e2e -- --project=workspaces
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, apiGet, apiPatch, apiDelete } from './helpers'

// Unique suffix so re-runs do not collide. Re-using a single RUN_ID across
// all tests in this file lets the cleanup test depend on what create made.
const RUN_ID = Date.now().toString(36)
const PUBLISHER_SLUG = `e2e-ws-${RUN_ID}`
const PUBLISHER_NAME = `E2E Workspaces ${RUN_ID}`
const WORKSPACE_SLUG = `team-${RUN_ID}`
const WORKSPACE_NAME = `Team ${RUN_ID}`
const SECOND_WS_SLUG = `team-b-${RUN_ID}`
const MCP_SLUG = `weather-${RUN_ID}`

test.describe.configure({ mode: 'serial' })

// Navigate to /admin so the SPA hydrates the oidc-client-ts session from the
// stored state and writes the access token into localStorage. Helpers in
// helpers.ts read it via page.evaluate; without a real navigation onto the
// SPA origin the read either hits about:blank (SecurityError) or finds an
// empty localStorage (token-not-found).
async function loadAdminSession(page: Page) {
  await page.goto('/admin')
  await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
}

test.describe('Workspaces API', () => {
  test.beforeEach(async ({ page }) => {
    await loadAdminSession(page)
  })

  test('admin creates publisher and workspace under it', async ({ page }) => {
    let resp = await apiPost(page, '/api/v1/publishers', {
      slug: PUBLISHER_SLUG,
      name: PUBLISHER_NAME,
    })
    expect(resp.status(), 'publisher create').toBe(201)

    resp = await apiPost(page, `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces`, {
      slug: WORKSPACE_SLUG,
      name: WORKSPACE_NAME,
      description: 'team owns claude',
    })
    expect(resp.status(), 'workspace create').toBe(201)
    const ws = await resp.json()
    expect(ws.slug).toBe(WORKSPACE_SLUG)
    expect(ws.name).toBe(WORKSPACE_NAME)
    expect(ws.description).toBe('team owns claude')
    // group_name is omitempty; absent on a freshly-created workspace.
    expect(ws.group_name ?? '').toBe('')
  })

  test('admin lists workspaces and gets the new one', async ({ page }) => {
    const list = await apiGet(page, `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces`)
    expect(list.status()).toBe(200)
    const body = await list.json()
    const slugs = (body.items as { slug: string }[]).map(w => w.slug)
    expect(slugs).toContain(WORKSPACE_SLUG)

    const detail = await apiGet(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${WORKSPACE_SLUG}`,
    )
    expect(detail.status()).toBe(200)
    const ws = await detail.json()
    expect(ws.slug).toBe(WORKSPACE_SLUG)
  })

  test('duplicate workspace slug under the same publisher returns 409', async ({ page }) => {
    const resp = await apiPost(page, `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces`, {
      slug: WORKSPACE_SLUG,
      name: 'duplicate',
    })
    expect(resp.status()).toBe(409)
  })

  test('admin sets and clears group_name on the workspace (ADR 0002)', async ({ page }) => {
    let resp = await apiPatch(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${WORKSPACE_SLUG}`,
      { group_name: 'claude-team' },
    )
    expect(resp.status(), 'set group').toBe(200)
    let ws = await resp.json()
    expect(ws.group_name).toBe('claude-team')

    // Clearing with empty string returns the workspace to admin-only.
    resp = await apiPatch(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${WORKSPACE_SLUG}`,
      { group_name: '' },
    )
    expect(resp.status(), 'clear group').toBe(200)
    ws = await resp.json()
    expect(ws.group_name ?? '').toBe('')
  })

  test('creating an MCP server lazily populates workspace_id', async ({ page }) => {
    // The flat create path remains admin-only and routes the new server to
    // the publisher's `default` workspace via ensureDefaultWorkspaceID.
    const resp = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUBLISHER_SLUG,
      slug: MCP_SLUG,
      name: 'Weather (e2e)',
    })
    expect(resp.status(), 'mcp create').toBe(201)

    // The publisher's default workspace must now exist.
    const def = await apiGet(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/default`,
    )
    expect(def.status(), 'default workspace exists after first create').toBe(200)

    // And the workspace-scoped servers list under default must include our
    // new server.
    const list = await apiGet(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/default/servers`,
    )
    expect(list.status()).toBe(200)
    const body = await list.json()
    const slugs = (body.items as { slug: string }[]).map(s => s.slug)
    expect(slugs).toContain(MCP_SLUG)
  })

  test('workspace-scoped server list isolates by workspace', async ({ page }) => {
    // Create a second workspace and confirm the existing server is NOT in it.
    const create = await apiPost(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces`,
      { slug: SECOND_WS_SLUG, name: 'Team B' },
    )
    expect(create.status()).toBe(201)

    const list = await apiGet(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${SECOND_WS_SLUG}/servers`,
    )
    expect(list.status()).toBe(200)
    const body = await list.json()
    const items = (body.items as unknown[]) ?? []
    expect(items.length).toBe(0)
  })

  test('deleting a non-empty workspace returns 409', async ({ page }) => {
    const resp = await apiDelete(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/default`,
    )
    expect(resp.status()).toBe(409)
  })

  test('cleanup: remove server, second workspace, original workspace, publisher', async ({ page }) => {
    // The bound workspace and the publisher's default must end up empty
    // before the publisher delete is allowed.
    const delServer = await apiDelete(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}`,
    )
    expect([200, 204], 'mcp delete').toContain(delServer.status())

    // Empty second workspace deletes cleanly.
    const delSecond = await apiDelete(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${SECOND_WS_SLUG}`,
    )
    expect(delSecond.status()).toBe(204)

    // The original workspace too.
    const delFirst = await apiDelete(
      page,
      `/api/v1/publishers/${PUBLISHER_SLUG}/workspaces/${WORKSPACE_SLUG}`,
    )
    expect(delFirst.status()).toBe(204)

    // Publisher delete cascades through the (now empty) default workspace.
    const delPub = await apiDelete(page, `/api/v1/publishers/${PUBLISHER_SLUG}`)
    expect(delPub.status()).toBe(204)
  })
})
