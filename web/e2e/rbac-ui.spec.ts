/**
 * rbac-ui.spec.ts
 *
 * End-to-end tests for the publisher-scoped RBAC admin UI:
 * the Groups pages (list / new / detail + members), the Users pages
 * (list / new / detail + Server-Admin toggle + set-password), and the
 * per-publisher Grants section on the publisher detail page.
 *
 * Drives the real browser DOM against the live stack — the unit tests
 * exercise these pages with a mocked API client, so this is what catches
 * handler/UI contract drift on the RBAC surface. Authorization *enforcement*
 * is covered separately by phase7-flows.spec.ts (via the API); this spec
 * covers the management UI a Server Admin uses to set those grants up.
 *
 * Prerequisites: same as the other admin specs (docker-compose dev stack;
 * the setup project populates e2e/.auth/*.json session cookies).
 *
 * Run:  npm run test:e2e -- --project=rbac-ui
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost, apiDelete } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `e2e-rbac-${RUN_ID}`
const GROUP = `team-${RUN_ID}`
const USER_EMAIL = `e2e-user-${RUN_ID}@example.com`
const MEMBER_EMAIL = `e2e-member-${RUN_ID}@example.com`

test.describe.configure({ mode: 'serial' })

test.describe('RBAC admin UI', () => {
  test('seed publisher via API', async ({ page }) => {
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    const r = await apiPost(page, '/api/v1/publishers', { slug: PUB, name: PUB })
    expect(r.status(), 'create publisher').toBe(201)
  })

  test('create a group through the New Group form', async ({ page }) => {
    await page.goto('/admin/groups/new')
    await page.fill('#slug', GROUP)
    await page.fill('#name', `Team ${RUN_ID}`)
    await page.fill('#description', 'Created by the rbac-ui e2e spec')
    await page.getByRole('button', { name: /create group/i }).click()

    // Lands back on the list, which now includes the new group.
    await page.waitForURL(/\/admin\/groups$/)
    const row = page.locator('tr', { hasText: GROUP })
    await expect(row).toBeVisible({ timeout: 15_000 })
  })

  test('add and remove a member on the group detail page', async ({ page }) => {
    await page.goto('/admin/groups')
    await page.locator('tr', { hasText: GROUP }).getByRole('link', { name: /manage/i }).click()

    await page.waitForURL(new RegExp(`/admin/groups/${GROUP}$`))
    await expect(page.getByRole('heading', { name: /members/i })).toBeVisible({ timeout: 15_000 })

    // Add a member by email — an unknown email provisions an invited user.
    await page.fill('#member-email', MEMBER_EMAIL)
    await page.getByRole('button', { name: /^add$/i }).click()

    const memberRow = page.locator('tr', { hasText: MEMBER_EMAIL })
    await expect(memberRow).toBeVisible({ timeout: 10_000 })

    // Remove it again — the row leaves the members table.
    await memberRow.getByRole('button', { name: new RegExp(`remove ${MEMBER_EMAIL}`, 'i') }).click()
    await expect(memberRow).toBeHidden({ timeout: 10_000 })
  })

  test('create a user through the New User form', async ({ page }) => {
    await page.goto('/admin/users/new')
    await page.fill('#email', USER_EMAIL)
    await page.fill('#display_name', `E2E User ${RUN_ID}`)
    // Leave password blank → invited user (no credential yet).
    await page.getByRole('button', { name: /create user/i }).click()

    await page.waitForURL(/\/admin\/users$/)
    await expect(page.locator('tr', { hasText: USER_EMAIL })).toBeVisible({ timeout: 15_000 })
  })

  test('toggle Server Admin and set a password on the user detail page', async ({ page }) => {
    await page.goto('/admin/users')
    await page.locator('tr', { hasText: USER_EMAIL }).getByRole('link', { name: /manage/i }).click()
    await page.waitForURL(/\/admin\/users\/[^/]+$/)
    await expect(page.getByRole('heading', { name: /actions/i })).toBeVisible({ timeout: 15_000 })

    // Grant Server Admin → the badge appears (the patch invalidates the query).
    await page.getByRole('button', { name: /grant server admin/i }).click()
    await expect(page.getByText('Server Admin', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Revoke it again → the badge disappears.
    await page.getByRole('button', { name: /revoke server admin/i }).click()
    await expect(page.getByText('Server Admin', { exact: true })).toBeHidden({ timeout: 10_000 })

    // Set a local password. The page doesn't refetch on success, so reload and
    // assert the durable "Local password: set" state rather than the toast.
    await page.fill('#password', 'e2e-password-123')
    await page.getByRole('button', { name: /set password/i }).click()
    await page.reload()
    const localPwTerm = page.getByText('Local password', { exact: true })
    await expect(localPwTerm).toBeVisible({ timeout: 15_000 })
    await expect(localPwTerm.locator('xpath=following-sibling::dd[1]')).toHaveText(/set/i)
  })

  test('grant and revoke a publisher role on the publisher detail page', async ({ page }) => {
    await page.goto(`/admin/publishers/${PUB}`)
    await expect(page.getByRole('heading', { name: /role grants/i })).toBeVisible({ timeout: 15_000 })

    // Grant the group an Editor role on this publisher.
    await page.selectOption('#principal-type', 'group')
    await page.selectOption('#principal-id', { label: GROUP })
    await page.selectOption('#grant-role', 'editor')
    await page.getByRole('button', { name: /^grant$/i }).click()

    // The grant row appears with the group label and the editor role.
    const grantRow = page.locator('tr', { hasText: GROUP })
    await expect(grantRow).toBeVisible({ timeout: 10_000 })
    await expect(grantRow.getByText('editor', { exact: true })).toBeVisible()

    // Revoke it → the row leaves the grants table.
    await grantRow.getByRole('button', { name: /revoke/i }).click()
    await expect(grantRow).toBeHidden({ timeout: 10_000 })
  })

  test('cleanup', async ({ page }: { page: Page }) => {
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })
    // Best-effort — don't assert; a failure above may leave partial state and
    // the real failure should surface, not a cleanup error.
    await apiDelete(page, `/api/v1/groups/${GROUP}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUB}`).catch(() => {})
    // Users have no delete endpoint; the unique per-run email avoids collisions.
  })
})
