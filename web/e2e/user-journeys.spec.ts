/**
 * user-journeys.spec.ts
 *
 * End-to-end "what a real user does" scenarios against the live stack, focused
 * on create / edit / patch / delete flows and the UI/UX behaviours that unit
 * tests can only approximate:
 *
 *   1. Full MCP server lifecycle through the UI (create → publish → edit →
 *      add a second version with the rich fields → publish → deprecate →
 *      delete), with API assertions that the rich version fields persisted.
 *   2. Admin list scoping to the publisher selected in the switcher.
 *   3. The create form pre-selecting the switcher's current publisher.
 *   4. "Load more" pagination appending rows instead of replacing them.
 *   5. The filtered empty state.
 *   6. Agent version rich fields (provider / docs / icon / skill examples).
 *   7. Settings rendered read-only for an editor-not-admin member.
 *   8. Bulk deprecate gated behind a confirmation dialog.
 *   9. Review workflow separation of duties: an Editor submits but cannot
 *      approve (UI + API 403); a distinct Reviewer approves from the queue.
 *
 * Identity is switched per-test via browser.newContext({ storageState }) like
 * phase7-flows.spec.ts — most steps run as the Server-Admin session; the
 * settings test also drives the author (editor-only) session.
 *
 * Run: npm run test:e2e -- --project=user-journeys
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiGet, confirmDialog } from './helpers'

const RUN = Date.now().toString(36)

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin' | 'author' | 'reviewer'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  await page.goto('/')
  return page
}

/** Create a publisher via the admin API; returns its slug + name. */
async function seedPublisher(page: Page, key: string): Promise<{ slug: string; name: string }> {
  const slug = `e2e-uj-${key}-${RUN}`
  const name = `E2E UJ ${key} ${RUN}`
  const res = await apiPost(page, '/api/v1/publishers', { slug, name })
  expect(res.status(), `create publisher ${slug}: ${await res.text()}`).toBe(201)
  return { slug, name }
}

/**
 * Grant the author's `anthropic-labs` group the Editor role on a publisher, so
 * the `author` session can author + submit there without being its Admin
 * (which keeps the review separation-of-duties tests honest).
 */
async function grantAuthorEditor(admin: Page, pubSlug: string): Promise<void> {
  const grp = await apiGet(admin, '/api/v1/groups/anthropic-labs')
  expect(grp.status(), 'anthropic-labs group exists').toBe(200)
  const grant = await apiPost(admin, `/api/v1/publishers/${pubSlug}/grants`, {
    principal_type: 'group',
    principal_id: (await grp.json()).id,
    role: 'editor',
  })
  expect(grant.status(), `grant editor: ${await grant.text()}`).toBe(201)
}

/** Seed a draft MCP version with a minimal stdio package. */
async function seedMcpVersion(admin: Page, pubSlug: string, slug: string, version: string): Promise<void> {
  const r = await apiPost(admin, `/api/v1/mcp/servers/${pubSlug}/${slug}/versions`, {
    version,
    runtime: 'stdio',
    protocol_version: '2025-03-26',
    packages: [{ registryType: 'npm', identifier: `@e2e/${slug}`, version, transport: { type: 'stdio' } }],
  })
  expect(r.status(), `seed version ${version}: ${await r.text()}`).toBe(201)
}

// ── 1. Full MCP server lifecycle through the UI ────────────────────────────────

test.describe('MCP server lifecycle (UI create → publish → edit → version → deprecate → delete)', () => {
  test('drives the whole lifecycle and persists rich version fields', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'mcp-life')
    const slug = `weather-${RUN}`


    // Scope the admin area to the publisher so the New form pre-selects it.
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: pub.name }).click()

    // CREATE — the publisher dropdown is pre-filled from the current scope.
    await page.goto('/admin/mcp/new')
    await expect(page.locator('#namespace-select')).toContainText(pub.slug)

    await page.fill('input[name="slug"]', slug)
    await page.fill('input[name="name"]', 'Weather Server')
    await page.fill('input[name="description"]', 'Forecasts via MCP.')
    await page.fill('input[name="version"]', '1.0.0')
    await page.fill('input[name="pkg_identifier"]', '@e2e/weather')
    await page.fill('input[name="pkg_version"]', '1.0.0')
    // "Publish version immediately" is checked by default → lands published.
    await page.getByRole('button', { name: 'Create MCP Server' }).click()

    await page.waitForURL(new RegExp(`/admin/mcp/${pub.slug}/${slug}`))
    await expect(page.getByText('Weather Server')).toBeVisible()
    await expect(page.getByText('published').first()).toBeVisible({ timeout: 10_000 })

    // EDIT metadata via the inline form.
    await page.getByRole('button', { name: /^edit$/i }).click()
    await page.fill('input[name="name"]', 'Weather Server v2')
    await page.fill('input[name="license"]', 'Apache-2.0')
    await page.getByRole('button', { name: /save changes/i }).click()
    await expect(page.getByText('Weather Server v2')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Apache-2.0')).toBeVisible()

    // ADD a second version through the New version form, exercising the rich
    // fields (capabilities JSON + package registry base URL + tools).
    await page.getByRole('button', { name: 'New version' }).click()
    // The form seeds itself from v1.0.0: patch-bumped version suggestion and
    // the previous package identifier, so authoring v2 is a delta, not a
    // blank slate.
    await expect(page.getByText(/Pre-filled from\s*v1\.0\.0/)).toBeVisible()
    await expect(page.locator('input[name="version"]')).toHaveValue('1.0.1')
    await expect(page.locator('input[name="pkg_identifier"]')).toHaveValue('@e2e/weather')
    await page.fill('input[name="version"]', '2.0.0')
    await page.fill('input[name="pkg_identifier"]', '@e2e/weather')
    await page.fill('input[name="pkg_version"]', '2.0.0')
    await page.fill('input[name="pkg_registry_base_url"]', 'https://registry.npmjs.org')
    await page.fill('textarea[name="capabilities"]', '{"tools":{"listChanged":true}}')
    // Author the tool through the dual-mode editor's structured Form tab (the
    // default). It serializes into a hidden `tools` input the submit reads.
    await page.getByRole('button', { name: /add tool/i }).click()
    await page.getByLabel('Tool 1 name').fill('forecast')
    await page.getByLabel('Tool 1 description').fill('Get a forecast')
    await page.getByRole('button', { name: 'Create version' }).click()
    await expect(page.getByRole('cell', { name: /v2\.0\.0/ })).toBeVisible({ timeout: 10_000 })

    // PUBLISH the new draft via its Publish button, confirming the themed
    // dialog (which replaced window.confirm).
    // `exact` avoids matching the LifecycleStepper's "Published" stage button.
    await page.getByRole('button', { name: 'Publish', exact: true }).first().click()
    await confirmDialog(page, 'Publish')
    // Two published versions now exist; assert the table shows ≥2 published.
    await expect
      .poll(async () => page.getByText('published').count(), { timeout: 10_000 })
      .toBeGreaterThanOrEqual(2)

    // VERIFY via the API that v2 persisted the rich fields the UI sent.
    const vres = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/2.0.0`)
    expect(vres.status()).toBe(200)
    const v2 = await vres.json()
    expect(v2.capabilities).toMatchObject({ tools: { listChanged: true } })
    expect(Array.isArray(v2.tools) && v2.tools.length).toBeTruthy()
    expect(v2.packages?.[0]?.registryBaseUrl).toBe('https://registry.npmjs.org')

    // DEPRECATE via the UI, confirming the themed dialog.
    await page.getByRole('button', { name: /^deprecate$/i }).click()
    await confirmDialog(page, 'Deprecate')
    await expect(page.getByText('deprecated').first()).toBeVisible({ timeout: 10_000 })

    // DELETE via the UI (Server-Admin escape hatch), confirming the dialog.
    await page.getByRole('button', { name: 'Delete', exact: true }).click()
    await confirmDialog(page, 'Delete')
    await page.waitForURL(/\/admin\/mcp(?:$|\?)/, { timeout: 10_000 })

    // GONE — the entry no longer resolves.
    const gone = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slug}`)
    expect(gone.status()).toBe(404)
  })
})

// ── 1b. Hosted (remote URL-only) MCP server ────────────────────────────────────

test.describe('Remote MCP server (URL-only) authoring', () => {
  test('creates a hosted server from the New form and surfaces the endpoint publicly', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'mcp-remote')
    const slug = `hosted-${RUN}`
    const endpoint = 'https://mcp.example.test/hosted/sse'

    // Scope the admin area so the New form pre-selects the publisher.
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: pub.name }).click()

    await page.goto('/admin/mcp/new')
    await expect(page.locator('#namespace-select')).toContainText(pub.slug)
    await page.fill('input[name="slug"]', slug)
    await page.fill('input[name="name"]', `Hosted Search ${RUN}`)
    await page.fill('input[name="version"]', '1.0.0')

    // The remote endpoint field only appears for non-stdio transports.
    await expect(page.locator('input[name="remote_url"]')).toHaveCount(0)
    await page.locator('#runtime-select').click()
    await page.getByRole('option', { name: /SSE/ }).click()
    await page.fill('input[name="remote_url"]', endpoint)
    // No package fields filled — a hosted server needs only the URL.
    await page.getByRole('button', { name: 'Create MCP Server' }).click()
    await page.waitForURL(new RegExp(`/admin/mcp/${pub.slug}/${slug}`))

    // The version persisted the remotes array (and no fabricated package).
    const vres = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0`)
    expect(vres.status()).toBe(200)
    const v = await vres.json()
    expect(v.remotes).toEqual([{ type: 'sse', url: endpoint }])
    expect(v.packages ?? []).toEqual([])

    // Make the entry public, then check the public detail page leads with
    // the connection surface for a hosted server.
    const vis = await apiPost(page, `/api/v1/mcp/servers/${pub.slug}/${slug}/visibility`, {
      visibility: 'public',
    })
    expect(vis.ok(), `make public: ${await vis.text()}`).toBeTruthy()

    await page.goto(`/mcp/${pub.slug}/${slug}`)
    await expect(page.getByText('Connection & Runtime')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(endpoint).first()).toBeVisible()

    // Installation tab renders the endpoint and the host-config generator.
    await page.getByRole('tab', { name: 'Installation' }).click()
    await expect(page.getByRole('heading', { name: 'Connection' })).toBeVisible()
    await expect(page.getByText('Host Configuration')).toBeVisible()
  })
})

// ── 2. List scoping to the switcher's selected publisher ───────────────────────

test.describe('Admin list scopes to the selected publisher', () => {
  test('switching publishers filters the MCP list to that publisher only', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const a = await seedPublisher(page, 'scope-a')
    const b = await seedPublisher(page, 'scope-b')

    const seedServer = async (ns: string, slug: string, name: string) => {
      const res = await apiPost(page, '/api/v1/mcp/servers', { namespace: ns, slug, name })
      expect(res.status(), `seed ${slug}: ${await res.text()}`).toBe(201)
    }
    await seedServer(a.slug, `srv-a-${RUN}`, `Scoped A ${RUN}`)
    await seedServer(b.slug, `srv-b-${RUN}`, `Scoped B ${RUN}`)

    // Scope to A → only A's server shows.
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: a.name }).click()
    await page.goto('/admin/mcp')
    await expect(page.getByText(`Scoped A ${RUN}`)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(`Scoped B ${RUN}`)).toHaveCount(0)

    // Scope to B → the inverse.
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: b.name }).click()
    await page.goto('/admin/mcp')
    await expect(page.getByText(`Scoped B ${RUN}`)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(`Scoped A ${RUN}`)).toHaveCount(0)
  })
})

// ── 3. Load more appends rows (pagination regression guard) ────────────────────

test.describe('Load more appends rows instead of replacing them', () => {
  test('clicking Load more keeps the first page visible and grows the list', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'paginate')

    // Seed 55 servers (one page is 50) concurrently.
    const total = 55
    const results = await Promise.all(
      Array.from({ length: total }, (_, i) =>
        apiPost(page, '/api/v1/mcp/servers', {
          namespace: pub.slug,
          slug: `pg-${RUN}-${String(i).padStart(2, '0')}`,
          name: `Paginate ${RUN} ${String(i).padStart(2, '0')}`,
        }),
      ),
    )
    for (const r of results) expect(r.status()).toBe(201)

    // Scope by explicit publisher filter so we read exactly this publisher.
    await page.goto(`/admin/mcp?namespace=${pub.slug}`)
    const rows = page.locator('table tbody tr')
    await expect(rows).toHaveCount(50, { timeout: 15_000 })

    // Capture a first-page row so we can prove it survives "Load more".
    const firstRowName = (await rows.first().locator('td').nth(1).innerText()).trim()

    await page.getByRole('button', { name: /load more/i }).click()

    // The list grew past one page AND the original first row is still present —
    // the old cursor-in-URL approach would have replaced page 1 entirely.
    // (Substring match: the name shares its cell with a hidden namespace/slug.)
    await expect.poll(async () => rows.count(), { timeout: 15_000 }).toBeGreaterThan(50)
    await expect(page.getByText(firstRowName).first()).toBeVisible()
  })
})

// ── 4. Filtered empty state ────────────────────────────────────────────────────

test.describe('Filtered empty state', () => {
  test('a no-match search shows the filter-specific empty message', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    await page.goto(`/admin/mcp?q=zzz-no-such-server-${RUN}`)
    await expect(page.getByText(/no servers match your filters/i)).toBeVisible({ timeout: 10_000 })
  })
})

// ── 5. Agent version rich fields through the UI ────────────────────────────────

test.describe('Agent version rich fields persist', () => {
  test('creates an agent version with provider / docs / icon / skill examples', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'agent-rich')
    const slug = `assistant-${RUN}`

    const res = await apiPost(page, '/api/v1/agents', {
      namespace: pub.slug,
      slug,
      name: `Assistant ${RUN}`,
    })
    expect(res.status(), `create agent: ${await res.text()}`).toBe(201)

    await page.goto(`/admin/agents/${pub.slug}/${slug}`)
    await page.getByRole('button', { name: 'New version' }).click()

    await page.fill('input[name="version"]', '1.0.0')
    await page.fill('input[name="endpoint_url"]', 'https://assistant.example.com/a2a')
    await page.fill('input[name="skill_id"]', 'summarize')
    await page.fill('input[name="skill_name"]', 'Summarize')
    await page.fill('input[name="skill_description"]', 'Summarizes text')
    await page.fill('input[name="skill_tags"]', 'text, nlp')
    await page.fill('textarea[name="skill_examples"]', 'Summarize this\nTLDR please')
    await page.fill('input[name="provider_organization"]', 'Acme Inc.')
    await page.fill('input[name="provider_url"]', 'https://acme.example.com')
    await page.fill('input[name="documentation_url"]', 'https://docs.example.com')
    await page.fill('input[name="icon_url"]', 'https://acme.example.com/icon.png')
    await page.fill('textarea[name="capabilities"]', '{"streaming":true}')
    await page.getByRole('button', { name: 'Create version' }).click()
    await expect(page.getByRole('cell', { name: /v1\.0\.0/ })).toBeVisible({ timeout: 10_000 })

    const vres = await apiGet(page, `/api/v1/agents/${pub.slug}/${slug}/versions/1.0.0`)
    expect(vres.status()).toBe(200)
    const v = await vres.json()
    expect(v.documentation_url).toBe('https://docs.example.com')
    expect(v.icon_url).toBe('https://acme.example.com/icon.png')
    expect(v.provider).toMatchObject({ organization: 'Acme Inc.' })
    expect(v.skills?.[0]?.examples).toContain('Summarize this')
    expect(v.capabilities).toMatchObject({ streaming: true })
  })
})

// ── 6. Settings read-only for an editor-not-admin member ───────────────────────

test.describe('Publisher Settings is read-only for a non-admin member', () => {
  test('an editor sees the settings but no Save, while an admin can edit', async ({ browser }) => {
    // Seed as Server Admin and grant the author's `anthropic-labs` group the
    // Editor (not Admin) role on a fresh publisher.
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'ro-settings')
    await grantAuthorEditor(admin, pub.slug)

    // The author is in anthropic-labs → Editor (not Admin) on this publisher.
    const author = await pageAs(browser, 'author')
    await author.goto('/admin')
    await author.getByRole('combobox', { name: /switch publisher/i }).click()
    await author.getByRole('option', { name: pub.name }).click()
    await author.goto('/admin/settings')

    // Read-only: name input present but disabled, no Save button, and the
    // explanatory copy is shown.
    const name = author.getByLabel(/^name/i)
    await expect(name).toBeVisible({ timeout: 10_000 })
    await expect(name).toBeDisabled()
    await expect(author.getByRole('button', { name: /save changes/i })).toHaveCount(0)
    await expect(author.getByText(/need the admin role/i)).toBeVisible()

    // Sanity: the Server Admin gets an editable form (Save present) on the same
    // publisher, proving the read-only state is permission-driven.
    await admin.goto('/admin')
    await admin.getByRole('combobox', { name: /switch publisher/i }).click()
    await admin.getByRole('option', { name: pub.name }).click()
    await admin.goto('/admin/settings')
    await expect(admin.getByRole('button', { name: /save changes/i })).toBeVisible({ timeout: 10_000 })
  })
})

// ── 7. Bulk deprecate is gated behind a confirmation dialog ────────────────────

test.describe('Bulk deprecate asks for confirmation', () => {
  test('declining keeps servers published; accepting deprecates them', async ({ browser }) => {
    const page = await pageAs(browser, 'admin')
    const pub = await seedPublisher(page, 'bulk-dep')

    // Seed two published servers (create → version → publish via the API).
    const slugs = [`bd-a-${RUN}`, `bd-b-${RUN}`]
    for (const s of slugs) {
      let r = await apiPost(page, '/api/v1/mcp/servers', {
        namespace: pub.slug,
        slug: s,
        name: `BulkDep ${s}`,
      })
      expect(r.status()).toBe(201)
      r = await apiPost(page, `/api/v1/mcp/servers/${pub.slug}/${s}/versions`, {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: `@e2e/${s}`, version: '1.0.0', transport: { type: 'stdio' } },
        ],
      })
      expect(r.status()).toBe(201)
      r = await apiPost(page, `/api/v1/mcp/servers/${pub.slug}/${s}/versions/1.0.0/publish`, {})
      expect(r.status()).toBe(200)
    }

    await page.goto(`/admin/mcp?namespace=${pub.slug}`)
    await expect(page.getByText(`BulkDep ${slugs[0]}`)).toBeVisible({ timeout: 10_000 })

    const selectAll = page.getByRole('checkbox', { name: /select all/i })

    // CANCEL the confirmation dialog → nothing changes.
    await selectAll.check()
    await page.getByRole('button', { name: /deprecate/i }).click()
    await confirmDialog(page, /^cancel$/i)
    // Both still report published via the API.
    for (const s of slugs) {
      const r = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${s}`)
      expect((await r.json()).status).toBe('published')
    }

    // CONFIRM the dialog → both deprecate.
    if (!(await selectAll.isChecked())) await selectAll.check()
    await page.getByRole('button', { name: /deprecate/i }).click()
    await confirmDialog(page, /^deprecate$/i)
    await expect
      .poll(
        async () => {
          const r = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slugs[0]}`)
          return (await r.json()).status
        },
        { timeout: 10_000 },
      )
      .toBe('deprecated')
    const r2 = await apiGet(page, `/api/v1/mcp/servers/${pub.slug}/${slugs[1]}`)
    expect((await r2.json()).status).toBe('deprecated')
  })
})

// ── 9. Review workflow separation of duties (Editor submits, Reviewer approves) ─

test.describe('Review workflow honors separation of duties', () => {
  test('an editor submits but cannot approve; a reviewer approves from the queue', async ({ browser }) => {
    // Admin seeds a publisher the author can edit, plus a server with a draft.
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'review')
    const slug = `srv-${RUN}`
    await grantAuthorEditor(admin, pub.slug)
    expect((await apiPost(admin, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'Review Srv' })).status()).toBe(201)
    await seedMcpVersion(admin, pub.slug, slug, '1.0.0')

    // EDITOR (author): submit the draft for review via the UI.
    const author = await pageAs(browser, 'author')
    await author.goto(`/admin/mcp/${pub.slug}/${slug}`)
    const row = author.locator('tr', { hasText: '1.0.0' })
    await row.getByRole('button', { name: /^submit$/i }).click()
    await confirmDialog(author, /^submit for review$/i)
    await expect(row.getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })

    // The editor has no approve affordance anywhere on the entry page…
    await expect(author.getByRole('button', { name: /^approve$/i })).toHaveCount(0)
    // …and the API enforces it too (Editor — even publisher Admin — is not a
    // Reviewer; separation of duties).
    const forbidden = await apiPost(
      author,
      `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0/approve`,
      {},
    )
    expect(forbidden.status(), 'editor approve is forbidden').toBe(403)
    // The editor's review queue offers nothing to approve.
    await author.goto('/admin/review')
    await expect(author.getByRole('button', { name: /^approve$/i })).toHaveCount(0)

    // REVIEWER: the pending version shows in the queue and approves to published.
    const reviewer = await pageAs(browser, 'reviewer')
    await reviewer.goto('/admin/review')
    const queueRow = reviewer
      .locator('li:not([data-sonner-toast])', { hasText: `${pub.slug}/${slug}` })
      .filter({ hasText: /MCP version/i })
    await expect(queueRow).toBeVisible({ timeout: 15_000 })
    await queueRow.getByRole('button', { name: /^approve$/i }).click()
    // Approvals confirm through the shared dialog (J2) before applying.
    await confirmDialog(reviewer, /approve/i)
    await expect(queueRow).toBeHidden({ timeout: 10_000 })

    // Confirm it actually published.
    const v = await apiGet(reviewer, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0`)
    expect(v.status()).toBe(200)
    expect((await v.json()).published_at).toBeTruthy()
  })
})

// ── 10. Review workflow — editor withdraw / conflict, reviewer reject ───────────

test.describe('Review workflow — withdraw, conflict, and cross-role reject', () => {
  test('an editor can withdraw a pending version back to draft', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'rev-withdraw')
    const slug = `srv-${RUN}`
    await grantAuthorEditor(admin, pub.slug)
    expect((await apiPost(admin, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'Withdraw Srv' })).status()).toBe(201)
    await seedMcpVersion(admin, pub.slug, slug, '1.0.0')

    const author = await pageAs(browser, 'author')
    await author.goto(`/admin/mcp/${pub.slug}/${slug}`)
    const row = author.locator('tr', { hasText: 'v1.0.0' })
    await row.getByRole('button', { name: /^submit$/i }).click()
    await confirmDialog(author, /^submit for review$/i)
    await expect(row.getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })

    // Withdraw returns it to draft: the Withdraw button gives way to Submit.
    await row.getByRole('button', { name: /^withdraw$/i }).click()
    await expect(row.getByRole('button', { name: /^submit$/i })).toBeVisible({ timeout: 10_000 })
    await expect(row.getByText(/pending review/i)).toHaveCount(0)

    const v = await apiGet(author, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0`)
    expect((await v.json()).review_state ?? 'none').not.toBe('pending_review')
  })

  test('submitting a second version while one is pending shows the conflict error', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'rev-conflict')
    const slug = `srv-${RUN}`
    await grantAuthorEditor(admin, pub.slug)
    expect((await apiPost(admin, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'Conflict Srv' })).status()).toBe(201)
    await seedMcpVersion(admin, pub.slug, slug, '1.0.0')
    await seedMcpVersion(admin, pub.slug, slug, '2.0.0')

    const author = await pageAs(browser, 'author')
    await author.goto(`/admin/mcp/${pub.slug}/${slug}`)
    // Submit the first version → pending.
    await author.locator('tr', { hasText: 'v1.0.0' }).getByRole('button', { name: /^submit$/i }).click()
    await confirmDialog(author, /^submit for review$/i)
    await expect(author.locator('tr', { hasText: 'v1.0.0' }).getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })
    // Only one version may be pending per entry, so submitting the second one
    // surfaces the friendly conflict message (mapped from review-already-pending).
    await author.locator('tr', { hasText: 'v2.0.0' }).getByRole('button', { name: /^submit$/i }).click()
    await confirmDialog(author, /^submit for review$/i)
    await expect(author.getByText(/already pending review/i)).toBeVisible({ timeout: 10_000 })

    // Clean up: withdraw the pending version so it doesn't linger in the shared
    // review queue (which other specs assert against).
    expect((await apiPost(author, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0/withdraw`, {})).status()).toBe(204)
  })

  test('a reviewer rejects a pending version with a reason from the queue', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    const pub = await seedPublisher(admin, 'rev-reject')
    const slug = `srv-${RUN}`
    await grantAuthorEditor(admin, pub.slug)
    expect((await apiPost(admin, '/api/v1/mcp/servers', { namespace: pub.slug, slug, name: 'Reject Srv' })).status()).toBe(201)
    await seedMcpVersion(admin, pub.slug, slug, '1.0.0')

    // Editor submits.
    const author = await pageAs(browser, 'author')
    await author.goto(`/admin/mcp/${pub.slug}/${slug}`)
    await author.locator('tr', { hasText: 'v1.0.0' }).getByRole('button', { name: /^submit$/i }).click()
    await confirmDialog(author, /^submit for review$/i)
    await expect(author.locator('tr', { hasText: 'v1.0.0' }).getByText(/pending review/i)).toBeVisible({ timeout: 10_000 })

    // Reviewer rejects from the queue with a reason.
    const reviewer = await pageAs(browser, 'reviewer')
    await reviewer.goto('/admin/review')
    const queueRow = reviewer
      .locator('li:not([data-sonner-toast])', { hasText: `${pub.slug}/${slug}` })
      .filter({ hasText: /MCP version/i })
    await expect(queueRow).toBeVisible({ timeout: 15_000 })
    await queueRow.getByRole('button', { name: /^reject$/i }).click()
    await queueRow.getByPlaceholder(/reason/i).fill('needs documentation')
    await queueRow.getByRole('button', { name: /confirm reject/i }).click()
    await expect(queueRow).toBeHidden({ timeout: 10_000 })

    // The rejection + reason are persisted on the version.
    const v = await apiGet(reviewer, `/api/v1/mcp/servers/${pub.slug}/${slug}/versions/1.0.0`)
    const body = await v.json()
    expect(body.review_state).toBe('rejected')
    expect(body.rejection_reason).toContain('needs documentation')
  })
})
