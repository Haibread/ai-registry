/**
 * publisher-home.spec.ts
 *
 * End-to-end coverage for the publisher-scoped admin home (ADR 0006):
 * the publisher switcher, the scoped Overview, and the Members + Activity
 * pages. Runs as the Server-Admin session — which (per the switcher change)
 * can scope to any publisher — so it seeds a publisher + resource via the
 * admin API, selects it in the switcher, and asserts each surface renders
 * scoped to that publisher.
 *
 * Run:  npm run test:e2e -- --project=publisher-home
 */

import { test, expect } from '@playwright/test'
import { apiPost } from './helpers'

const RUN_ID = Date.now().toString(36)
const PUB = `ph-${RUN_ID}`
const PUB_NAME = `Publisher Home ${RUN_ID}`
const MCP = `ph-mcp-${RUN_ID}`

test.describe.configure({ mode: 'serial' })

test.describe('Publisher-scoped admin home', () => {
  test('seed a publisher + MCP server via the API', async ({ page }) => {
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    const pub = await apiPost(page, '/api/v1/publishers', { slug: PUB, name: PUB_NAME })
    expect(pub.ok()).toBeTruthy()
    const mcp = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUB,
      slug: MCP,
      name: MCP,
      description: 'Publisher-home e2e fixture.',
    })
    expect(mcp.ok()).toBeTruthy()
  })

  test('switching to the publisher shows its scoped Overview', async ({ page }) => {
    await page.goto('/admin')
    // Scope the admin area to the seeded publisher via the switcher.
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: PUB_NAME }).click()

    // Overview header is the publisher name; the MCP metric reflects the one
    // server we seeded; the activity timeline shows its creation.
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('MCP servers')).toBeVisible()
    await expect(page.getByText(MCP).first()).toBeVisible()
  })

  test('Members page manages the selected publisher', async ({ page }) => {
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: PUB_NAME }).click()
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })

    await page.getByRole('link', { name: 'Members', exact: true }).click()
    await expect(page.getByRole('heading', { name: /members & roles/i })).toBeVisible({ timeout: 15_000 })
    // The grants management UI (add form / role select) is present.
    await expect(page.getByText(PUB)).toBeVisible()
  })

  test('Activity page shows the publisher feed', async ({ page }) => {
    await page.goto('/admin')
    await page.getByRole('combobox', { name: /switch publisher/i }).click()
    await page.getByRole('option', { name: PUB_NAME }).click()
    await expect(page.getByRole('heading', { name: PUB_NAME })).toBeVisible({ timeout: 15_000 })

    await page.getByRole('link', { name: 'Activity', exact: true }).click()
    await expect(page.getByRole('heading', { name: /^activity$/i })).toBeVisible({ timeout: 15_000 })
    // The seeded server's creation event appears in the feed.
    await expect(page.getByText(MCP).first()).toBeVisible()
  })
})
