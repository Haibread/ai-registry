/**
 * entry-change.spec.ts
 *
 * End-to-end tests for routing entry-level mutations (visibility, deprecate,
 * metadata edit) through the change-approval review queue.
 *
 * An Editor's request now ENQUEUES (202) instead of applying; a Reviewer
 * (here the Server Admin) approves it via the change-request endpoint, after
 * which the entry actually changes. Server Admins keep the immediate path
 * (200) as an escape hatch — covered by change-approval.spec.ts's 200 on
 * visibility.
 *
 * Identity is switched per-step via browser.newContext (like phase7-flows):
 *   - author   — Editor on the publisher (via the anthropic-core group grant)
 *   - admin    — Server Admin; approves and seeds
 *
 * Prerequisites:
 *   - the live stack is up (Postgres + server + web)
 *   - the setup project populated e2e/.auth/<role>.json
 *
 * Run:  npm run test:e2e -- --project=entry-change
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiGet, apiPatch, apiDelete } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `e2e-change-${RUN_ID}`
const MCP = `srv-${RUN_ID}`
const V1 = '1.0.0'

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin' | 'author'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  await page.goto('/')
  return page
}

test.describe('Entry-change review queue', () => {
  test('admin seeds publisher, editor grant, published server', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')

    let res = await apiPost(page, '/api/v1/publishers', { slug: PUB, name: `Entry Change ${RUN_ID}` })
    expect(res.status(), 'create publisher').toBe(201)

    // Authorize the anthropic-core group (the author's groups claim) to edit.
    let groupId: string
    const mkGroup = await apiPost(page, '/api/v1/groups', { slug: 'anthropic-core', name: 'Anthropic Core' })
    if (mkGroup.status() === 201) {
      groupId = (await mkGroup.json()).id
    } else {
      const existing = await apiGet(page, '/api/v1/groups/anthropic-core')
      expect(existing.status(), 'get existing group').toBe(200)
      groupId = (await existing.json()).id
    }
    res = await apiPost(page, `/api/v1/publishers/${PUB}/grants`, {
      principal_type: 'group',
      principal_id: groupId,
      role: 'editor',
    })
    expect(res.status(), 'grant editor').toBe(201)

    res = await apiPost(page, '/api/v1/mcp/servers', { namespace: PUB, slug: MCP, name: `Server ${MCP}` })
    expect(res.status(), 'create server').toBe(201)

    res = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${MCP}/versions`, {
      version: V1,
      runtime: 'stdio',
      packages: [{ registryType: 'npm', identifier: '@e2e/pkg', version: V1, transport: { type: 'stdio' } }],
      capabilities: { tools: [] },
      protocol_version: '2024-11-05',
    })
    expect(res.status(), 'create version').toBe(201)

    // Submit + approve the version so the entry is published (eligible for
    // visibility-to-public and deprecate).
    res = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${MCP}/versions/${V1}/submit`, {})
    expect(res.status(), 'submit version').toBe(204)
    res = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${MCP}/versions/${V1}/approve`, { revision: 1 })
    expect(res.status(), 'approve version').toBe(204)
  })

  test('editor visibility-to-public enqueues (202), entry stays private', async ({ browser }) => {
    const author = await pageAs(browser, 'author')
    const res = await apiPost(author, `/api/v1/mcp/servers/${PUB}/${MCP}/visibility`, { visibility: 'public' })
    expect(res.status(), 'editor enqueue visibility').toBe(202)
    const body = (await res.json()) as { status: string; change_id: string }
    expect(body.status).toBe('pending_review')
    expect(body.change_id).toBeTruthy()

    // Still private until approved (read as admin who can see private detail).
    const admin = await pageAs(browser, 'admin')
    const detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).visibility).toBe('private')
  })

  test('a second pending change on the same entry is 409 change-already-pending', async ({ browser }) => {
    const author = await pageAs(browser, 'author')
    const res = await apiPost(author, `/api/v1/mcp/servers/${PUB}/${MCP}/deprecate`, {})
    expect(res.status(), 'second pending change').toBe(409)
    const body = (await res.json()) as { type: string }
    expect(body.type).toMatch(/change-already-pending$/)
  })

  test('the pending change shows in the review queue', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const res = await apiGet(admin, '/api/v1/review-queue')
    expect(res.status()).toBe(200)
    const body = (await res.json()) as { items: { kind: string; action?: string; entry_slug: string }[] }
    const item = body.items.find(i => i.kind === 'mcp_change' && i.entry_slug === MCP)
    expect(item, 'mcp_change queue item').toBeDefined()
    expect(item?.action).toBe('visibility')
  })

  test('reviewer approve applies the change (entry goes public)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const res = await apiPost(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/change-request/approve`, { revision: 1 })
    expect(res.status(), 'approve change').toBe(204)

    const detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).visibility).toBe('public')
  })

  test('editor metadata edit enqueues and can be withdrawn', async ({ browser }) => {
    const author = await pageAs(browser, 'author')
    let res = await apiPatch(author, `/api/v1/mcp/servers/${PUB}/${MCP}`, { name: 'Renamed by editor' })
    expect(res.status(), 'enqueue metadata edit').toBe(202)

    // The edit is not applied yet.
    const admin = await pageAs(browser, 'admin')
    let detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).name).toBe(`Server ${MCP}`)

    // Author withdraws it; the entry is unchanged and the slot is freed.
    res = await apiPost(author, `/api/v1/mcp/servers/${PUB}/${MCP}/change-request/withdraw`, {})
    expect(res.status(), 'withdraw').toBe(204)

    detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).pending_change ?? null, 'no pending change after withdraw').toBeNull()
  })

  test('deprecate → republish round-trip through the queue', async ({ browser }) => {
    // Admin deprecates immediately (escape hatch).
    const admin = await pageAs(browser, 'admin')
    let res = await apiPost(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/deprecate`, {})
    expect(res.status(), 'admin deprecate').toBe(200)

    // Editor requests a republish: enqueued (202), entry stays deprecated.
    const author = await pageAs(browser, 'author')
    res = await apiPost(author, `/api/v1/mcp/servers/${PUB}/${MCP}/undeprecate`, {})
    expect(res.status(), 'editor enqueue republish').toBe(202)
    let detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).status).toBe('deprecated')

    // The queue carries it as an undeprecation; approval applies it.
    const queue = await apiGet(admin, '/api/v1/review-queue')
    const item = ((await queue.json()) as { items: { kind: string; action?: string; entry_slug: string }[] })
      .items.find(i => i.kind === 'mcp_change' && i.entry_slug === MCP)
    expect(item?.action, 'queued undeprecation').toBe('undeprecation')

    res = await apiPost(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/change-request/approve`, { revision: 1 })
    expect(res.status(), 'approve republish').toBe(204)
    detail = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    expect((await detail.json()).status).toBe('published')
  })

  test('the detail page offers Republish on a deprecated entry (admin immediate)', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const res = await apiPost(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/deprecate`, {})
    expect(res.status(), 're-deprecate').toBe(200)

    await admin.goto(`/admin/mcp/${PUB}/${MCP}`)
    const republish = admin.getByRole('button', { name: 'Republish' })
    await expect(republish, 'Republish action visible on deprecated entry').toBeVisible()
    await republish.click()

    // Admin path applies immediately; the action disappears with the status flip.
    await expect(admin.getByText('Server republished')).toBeVisible()
    await expect(republish).toBeHidden()
  })

  test('cleanup', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    // Force-delete the server (admin escape hatch) then drop the publisher.
    await apiDelete(admin, `/api/v1/mcp/servers/${PUB}/${MCP}`)
    const res = await apiDelete(admin, `/api/v1/publishers/${PUB}`)
    expect([200, 204], 'publisher delete').toContain(res.status())
  })
})
