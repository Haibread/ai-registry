/**
 * phase7-flows.spec.ts
 *
 * End-to-end proof that the dev realm produces JWTs with the right
 * `groups[]` claim and that the server middleware honours them. Covers
 * the three Phase 7 authorization paths that admin@ alone never
 * exercises:
 *
 *   - author@example.com    (groups: anthropic-core, anthropic-labs)
 *                           — RequireWorkspaceWrite: submit + create version
 *   - reviewer@example.com  (group: registry-reviewers)
 *                           — RequireReviewer: approve a pending submission
 *   - user@example.com      (no roles, no groups)
 *                           — 403 baseline with the workspace-group message
 *
 * Storage states are minted by global.setup.ts. The spec uses
 * browser.newContext({ storageState }) per role rather than a single
 * project-level storageState — by design, it's the only spec that
 * switches identity mid-run.
 *
 * Prerequisites (same as the other admin specs):
 *   - docker compose up; Keycloak has imported the dev realm
 *   - AUTH_STORAGE=local in deploy/.env (Playwright reads localStorage)
 *
 * Run: npm run test:e2e -- --project=phase7-flows
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiPatch, apiGet, apiDelete } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `e2e-phase7-${RUN_ID}`
const MCP = `srv-${RUN_ID}`
const V1 = '1.0.0'

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin' | 'author' | 'reviewer' | 'user'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  // The helpers extract the OIDC token from localStorage via
  // page.evaluate, which requires the page to have an origin. Land on
  // the homepage — RequireAuth doesn't bounce off it for any role.
  await page.goto('/')
  return page
}

test.describe('Phase 7: group-based authorization end-to-end', () => {
  test('admin seeds publisher, workspace bound to anthropic-core, server, draft version', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')

    let res = await apiPost(page, '/api/v1/publishers', {
      slug: PUB,
      name: `E2E Phase 7 ${RUN_ID}`,
      verified: false,
    })
    expect(res.status(), 'create publisher').toBe(201)

    // The flat POST lazily creates the `default` workspace under the
    // publisher. CreateWorkspaceRequest doesn't accept `group_name`
    // (see openapi.yaml), so the binding has to land via PATCH after
    // the workspace exists.
    res = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB,
      slug: MCP,
      name: `Server ${MCP}`,
    })
    expect(res.status(), 'create server').toBe(201)

    res = await apiPatch(
      page,
      `/api/v1/publishers/${PUB}/workspaces/default`,
      { group_name: 'anthropic-core' },
    )
    expect(res.status(), 'bind default workspace to anthropic-core').toBe(200)

    res = await apiPost(page, `/api/v1/mcp/servers/${PUB}/${MCP}/versions`, {
      version: V1,
      runtime: 'stdio',
      packages: [
        {
          registryType: 'npm',
          identifier: '@e2e/pkg',
          version: V1,
          transport: { type: 'stdio' },
        },
      ],
      capabilities: { tools: [] },
      protocol_version: '2024-11-05',
    })
    expect(res.status(), 'create draft version').toBe(201)
  })

  test('author (anthropic-core group) submits the draft → pending_review', async ({ browser }) => {
    const page = await pageAs(browser, 'author')

    const submit = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/${V1}/submit`,
      {},
    )
    expect(submit.status(), `submit: ${await submit.text()}`).toBe(204)

    // Confirm the state transition via an admin read so we're checking
    // the server's stored value, not the author's view of it.
    const admin = await pageAs(browser, 'admin')
    const get = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/versions/${V1}`)
    expect(get.status()).toBe(200)
    const body = await get.json()
    expect(body.review_state, 'review_state after author submit').toBe('pending_review')
  })

  test('reviewer (registry-reviewers group) approves → published', async ({ browser }) => {
    const page = await pageAs(browser, 'reviewer')

    // approve takes the current revision for optimistic concurrency;
    // a fresh submit puts the version at revision 1.
    const approve = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions/${V1}/approve`,
      { revision: 1 },
    )
    expect(approve.status(), `approve: ${await approve.text()}`).toBe(204)

    // The canonical "is it published?" signal is published_at being
    // set on the version — same as change-approval.spec.ts. The
    // status field carries the post-publish lifecycle value
    // ("active"), which is a separate axis from review_state.
    const admin = await pageAs(browser, 'admin')
    const list = await apiGet(admin, `/api/v1/mcp/servers/${PUB}/${MCP}/versions`)
    expect(list.status()).toBe(200)
    const body = await list.json()
    const v = (body.items as { version: string; published_at?: string; review_state?: string }[]).find(
      it => it.version === V1,
    )
    expect(v, `${V1} in version list`).toBeDefined()
    expect(v?.published_at, `${V1} published_at after approve`).toBeTruthy()
    expect(v?.review_state ?? 'none', 'review_state after approve').toBe('none')
  })

  test('user (no roles, no groups) is forbidden from creating a version under the same workspace', async ({ browser }) => {
    const page = await pageAs(browser, 'user')

    // user@example.com is authenticated but carries an empty groups[]
    // claim and no admin realm role, so RequireWorkspaceWrite must
    // 403 with the workspace-group message.
    const v2 = '2.0.0'
    const res = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUB}/${MCP}/versions`,
      {
        version: v2,
        runtime: 'stdio',
        packages: [
          {
            registryType: 'npm',
            identifier: '@e2e/pkg',
            version: v2,
            transport: { type: 'stdio' },
          },
        ],
        capabilities: { tools: [] },
        protocol_version: '2024-11-05',
      },
    )
    expect(res.status(), 'user create version should be forbidden').toBe(403)

    const body = await res.json()
    const message = String(body.detail ?? body.title ?? '')
    expect(message, `403 problem+json body: ${JSON.stringify(body)}`).toMatch(
      /workspace.*group membership|insufficient permissions/i,
    )
  })

  test.afterAll(async ({ browser }) => {
    // Best-effort cleanup so reruns don't accumulate. Don't assert —
    // a failed test above may leave partial state, and the run still
    // surfaces the real failure above this.
    const page = await pageAs(browser, 'admin')
    await apiDelete(page, `/api/v1/mcp/servers/${PUB}/${MCP}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB}/workspaces/default`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB}`).catch(() => {})
  })
})
