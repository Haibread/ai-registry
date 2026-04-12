/**
 * admin.spec.ts
 *
 * End-to-end tests for the admin UI flows:
 *
 *  1. Publisher CRUD — create, edit, then delete a publisher.
 *  2. MCP Server CRUD — create, edit, visibility toggle, deprecate, delete.
 *  3. Agent CRUD — create, edit, visibility toggle, delete.
 *
 * These tests run sequentially (workers: 1) because they share mutable DB
 * state. The setup project must run first to populate e2e/.auth/admin.json.
 */

import { test, expect, type Page } from '@playwright/test'
import { apiPost } from './helpers'

// Unique suffix to avoid collisions between test runs.
const RUN_ID = Date.now().toString(36)
const PUBLISHER_SLUG = `e2e-pub-${RUN_ID}`
const PUBLISHER_NAME = `E2E Publisher ${RUN_ID}`
const MCP_SLUG = `e2e-mcp-${RUN_ID}`
const MCP_NAME = `E2E MCP ${RUN_ID}`
const AGENT_SLUG = `e2e-agent-${RUN_ID}`
const AGENT_NAME = `E2E Agent ${RUN_ID}`

// ── helpers ───────────────────────────────────────────────────────────────────

async function goTo(page: Page, path: string) {
  await page.goto(path)
  // Don't use networkidle — automaticSilentRenew and TanStack Query retries
  // keep the network active and prevent networkidle from ever resolving.
  await page.waitForLoadState('domcontentloaded')
}

// Select a Radix UI <Select> option by its trigger element id and partial text.
// Waits for the option to appear (publishers may load after the trigger is clicked).
async function selectOption(page: Page, triggerId: string, optionText: string | RegExp) {
  await page.locator(`#${triggerId}`).click()
  await expect(page.getByRole('option', { name: optionText })).toBeVisible({ timeout: 15_000 })
  await page.getByRole('option', { name: optionText }).click()
}

// ── Publisher ─────────────────────────────────────────────────────────────────

test.describe('Admin: Publisher CRUD', () => {
  test('create a publisher', async ({ page }) => {
    await goTo(page, '/admin/publishers/new')

    await page.fill('input[name="slug"]', PUBLISHER_SLUG)
    await page.fill('input[name="name"]', PUBLISHER_NAME)
    await page.click('button[type="submit"]')

    await page.waitForURL(/\/admin\/publishers/)
    await expect(page.getByText(PUBLISHER_NAME)).toBeVisible()
  })

  test('edit a publisher name', async ({ page }) => {
    await goTo(page, `/admin/publishers/${PUBLISHER_SLUG}`)

    await page.click('button:has-text("Edit")')
    await page.fill('input[name="name"]', `${PUBLISHER_NAME} edited`)
    await page.click('button:has-text("Save changes")')

    await expect(page.getByText(`${PUBLISHER_NAME} edited`)).toBeVisible({ timeout: 10_000 })
  })
})

// ── MCP Server ────────────────────────────────────────────────────────────────

test.describe('Admin: MCP Server CRUD', () => {
  test('create an MCP server', async ({ page }) => {
    await goTo(page, '/admin/mcp/new')

    // Namespace is a Radix <Select> — publishers load from the API after auth.
    await expect(page.locator('#namespace-select')).toBeVisible({ timeout: 10_000 })
    await selectOption(page, 'namespace-select', new RegExp(PUBLISHER_SLUG))

    await page.fill('input[name="slug"]', MCP_SLUG)
    await page.fill('input[name="name"]', MCP_NAME)
    await page.fill('input[name="description"]', 'An E2E test MCP server.')
    await page.click('button[type="submit"]')

    await page.waitForURL(new RegExp(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`))
    await expect(page.getByText(MCP_NAME)).toBeVisible()
  })

  test('MCP server detail page shows draft status', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText('draft').first()).toBeVisible()
    await expect(page.getByText('private').first()).toBeVisible()
  })

  test('edit MCP server metadata', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    await page.click('button:has-text("Edit")')
    await page.fill('input[name="name"]', `${MCP_NAME} edited`)
    await page.fill('input[name="license"]', 'MIT')
    await page.click('button:has-text("Save changes")')

    await expect(page.getByText(`${MCP_NAME} edited`)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('MIT')).toBeVisible({ timeout: 10_000 })
  })

  test('toggle MCP server visibility to public', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    await page.click('button:has-text("Make public")')
    await expect(page.getByText('public').first()).toBeVisible()
  })

  test('toggle MCP server visibility back to private', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await page.click('button:has-text("Make private")')
    await expect(page.getByText('private').first()).toBeVisible()
  })

  test('publish a version via API, then deprecate via UI', async ({ page }) => {
    // Navigate first so localStorage is accessible for apiPost.
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    // Create a version via the API.
    const createRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`,
      {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@e2e/test-server', version: '1.0.0', transport: { type: 'stdio' } },
        ],
      }
    )
    expect(createRes.ok()).toBeTruthy()

    // Publish it.
    const publishRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`,
      {}
    )
    expect(publishRes.ok()).toBeTruthy()

    // Reload the detail page — Deprecate button should now appear.
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText('published').first()).toBeVisible()
    await expect(page.getByRole('button', { name: 'Deprecate' })).toBeVisible()

    // Deprecate via UI.
    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Deprecate")')
    await expect(page.getByText('deprecated').first()).toBeVisible()
  })

  test('delete an MCP server', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Delete")')

    // Navigated back to the list.
    await page.waitForURL(/\/admin\/mcp$/)
    await expect(page.getByText(MCP_NAME)).not.toBeVisible()
  })
})

// ── Agent ─────────────────────────────────────────────────────────────────────

test.describe('Admin: Agent CRUD', () => {
  test('create an agent', async ({ page }) => {
    await goTo(page, '/admin/agents/new')

    // Namespace is a Radix <Select> — publishers load from the API after auth.
    await expect(page.locator('#namespace-select')).toBeVisible({ timeout: 10_000 })
    await selectOption(page, 'namespace-select', new RegExp(PUBLISHER_SLUG))

    await page.fill('input[name="slug"]', AGENT_SLUG)
    await page.fill('input[name="name"]', AGENT_NAME)
    await page.fill('input[name="description"]', 'An E2E test agent.')
    await page.click('button[type="submit"]')

    await page.waitForURL(new RegExp(`/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`))
    await expect(page.getByText(AGENT_NAME)).toBeVisible()
  })

  test('agent detail page shows draft status', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByText('draft').first()).toBeVisible()
    await expect(page.getByText('private').first()).toBeVisible()
  })

  test('edit agent metadata', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)

    await page.click('button:has-text("Edit")')
    await page.fill('input[name="name"]', `${AGENT_NAME} edited`)
    await page.click('button:has-text("Save changes")')

    await expect(page.getByText(`${AGENT_NAME} edited`)).toBeVisible({ timeout: 10_000 })
  })

  test('toggle agent visibility to public', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await page.click('button:has-text("Make public")')
    await expect(page.getByText('public').first()).toBeVisible()
  })

  test('delete an agent', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)

    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Delete")')

    await page.waitForURL(/\/admin\/agents$/)
    await expect(page.getByText(`${AGENT_NAME} edited`, { exact: true })).not.toBeVisible({ timeout: 10_000 })
  })
})

// ── Publisher delete (must be last — publisher owns nothing after server/agent deleted) ──

test.describe('Admin: Publisher delete', () => {
  test('delete a publisher once its entries are gone', async ({ page }) => {
    await goTo(page, `/admin/publishers/${PUBLISHER_SLUG}`)

    // Wait for the publisher data to load before looking for the Delete button.
    // The query is gated on accessToken; auth hydration from storageState is async.
    await expect(page.getByText(`${PUBLISHER_NAME} edited`, { exact: true })).toBeVisible({ timeout: 15_000 })

    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Delete")')

    await page.waitForURL(/\/admin\/publishers$/)
    await expect(page.getByText(`${PUBLISHER_NAME} edited`, { exact: true })).not.toBeVisible({ timeout: 10_000 })
  })
})
