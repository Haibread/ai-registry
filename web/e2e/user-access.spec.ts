/**
 * user-access.spec.ts
 *
 * E2E coverage for the per-user access view on the admin user detail page
 * (GET /api/v1/users/{id}/grants): the Access section aggregates the user's
 * direct grants and the grants inherited through local group membership, with
 * scope and provenance links.
 *
 * Run: npm run test:e2e -- user-access
 */

import { test, expect, type Browser, type Page } from '@playwright/test'
import { apiPost, apiPut, apiPatch, apiDelete, apiGet } from './helpers'

const RUN = Date.now().toString(36)

test.describe.configure({ mode: 'serial' })

async function pageAs(browser: Browser, role: 'admin'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: `e2e/.auth/${role}.json` })
  const page = await ctx.newPage()
  await page.goto('/')
  return page
}

const PUB = `e2e-access-${RUN}`
const GROUP = `e2e-access-team-${RUN}`
const EMAIL = `e2e-access-${RUN}@example.com`

let userId = ''
let globalGrantId = ''

test.describe('Per-user access view', () => {
  test('seed', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')

    const pub = await apiPost(admin, '/api/v1/publishers', { slug: PUB, name: `E2E Access ${RUN}` })
    expect(pub.status(), `create publisher: ${await pub.text()}`).toBe(201)

    const user = await apiPost(admin, '/api/v1/users', { email: EMAIL, display_name: 'Access E2E' })
    expect(user.status(), `create user: ${await user.text()}`).toBe(201)
    userId = (await user.json()).id

    const group = await apiPost(admin, '/api/v1/groups', { slug: GROUP, name: 'Access E2E Team' })
    expect(group.status(), `create group: ${await group.text()}`).toBe(201)
    const groupId = (await group.json()).id
    const member = await apiPut(admin, `/api/v1/groups/${GROUP}/members/${EMAIL}`, {})
    expect(member.status(), `add member: ${await member.text()}`).toBe(200)

    // Direct per-publisher grant + group grant + direct global grant.
    const direct = await apiPost(admin, `/api/v1/publishers/${PUB}/grants`, {
      principal_type: 'user', principal_id: userId, role: 'viewer',
    })
    expect(direct.status(), `direct grant: ${await direct.text()}`).toBe(201)
    const viaGroup = await apiPost(admin, `/api/v1/publishers/${PUB}/grants`, {
      principal_type: 'group', principal_id: groupId, role: 'editor',
    })
    expect(viaGroup.status(), `group grant: ${await viaGroup.text()}`).toBe(201)
    const global = await apiPost(admin, '/api/v1/grants', {
      principal_type: 'user', principal_id: userId, role: 'reviewer',
    })
    expect(global.status(), `global grant: ${await global.text()}`).toBe(201)
    globalGrantId = (await global.json()).id
  })

  test('the access section lists direct, group-derived, and global grants', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto(`/admin/users/${userId}`)

    await expect(admin.getByRole('heading', { name: 'Access', exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(admin.getByRole('link', { name: 'What can each role do?' }))
      .toHaveAttribute('href', '/admin/help/roles')

    const rows = admin.locator('tbody tr')
    await expect(rows).toHaveCount(3)

    // Global direct grant: all-publishers scope, no group.
    const globalRow = rows.filter({ hasText: 'reviewer' })
    await expect(globalRow).toContainText('All publishers')
    await expect(globalRow).toContainText('direct')

    // Group-derived grant: publisher scope link + group provenance link.
    const groupRow = rows.filter({ hasText: 'editor' })
    await expect(groupRow.getByRole('link', { name: `E2E Access ${RUN}` }))
      .toHaveAttribute('href', `/admin/publishers/${PUB}`)
    await expect(groupRow.getByRole('link', { name: 'Access E2E Team' }))
      .toHaveAttribute('href', `/admin/groups/${GROUP}`)

    // Direct per-publisher grant. ("viewer" is a substring of "reviewer",
    // so match the role badge exactly rather than the row text.)
    const directRow = rows.filter({ has: admin.getByText('viewer', { exact: true }) })
    await expect(directRow).toContainText('direct')

    // The group link drills into the group detail page.
    await groupRow.getByRole('link', { name: 'Access E2E Team' }).click()
    await expect(admin).toHaveURL(`/admin/groups/${GROUP}`)
  })

  test('granting Server Admin updates the access summary', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    await admin.goto(`/admin/users/${userId}`)
    await expect(admin.getByRole('heading', { name: 'Access', exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(admin.getByText(/full access to every publisher/i)).toBeHidden()

    await admin.getByRole('button', { name: 'Grant Server Admin' }).click()
    const dialog = admin.locator('dialog', { hasText: /grant server admin/i })
    await dialog.getByRole('button', { name: 'Grant Server Admin' }).click()

    await expect(admin.getByText(/full access to every publisher/i)).toBeVisible({ timeout: 10_000 })
  })

  test('cleanup', async ({ browser }) => {
    const admin = await pageAs(browser, 'admin')
    // Publisher deletion cascades its grants; the group cascades its own.
    await apiDelete(admin, `/api/v1/grants/${globalGrantId}`)
    await apiDelete(admin, `/api/v1/groups/${GROUP}`)
    const r = await apiDelete(admin, `/api/v1/publishers/${PUB}`)
    expect([200, 204]).toContain(r.status())
    // No DELETE /users endpoint — park the seeded user as disabled instead.
    await apiPatch(admin, `/api/v1/users/${userId}`, { disabled: true })
    const verify = await apiGet(admin, `/api/v1/users/${userId}/grants`)
    expect(verify.status()).toBe(200)
    expect((await verify.json()).items).toHaveLength(0)
  })
})
