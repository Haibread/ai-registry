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
  await page.waitForLoadState('networkidle')
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

    await page.waitForLoadState('networkidle')
    await expect(page.getByText(`${PUBLISHER_NAME} edited`)).toBeVisible()
  })
})

// ── MCP Server ────────────────────────────────────────────────────────────────

test.describe('Admin: MCP Server CRUD', () => {
  test('create an MCP server', async ({ page }) => {
    await goTo(page, '/admin/mcp/new')

    await page.fill('input[name="namespace"]', PUBLISHER_SLUG)
    await page.fill('input[name="slug"]', MCP_SLUG)
    await page.fill('input[name="name"]', MCP_NAME)
    await page.fill('textarea[name="description"], input[name="description"]', 'An E2E test MCP server.')
    await page.click('button[type="submit"]')

    await page.waitForURL(/\/admin\/mcp/)
    await expect(page.getByText(MCP_NAME)).toBeVisible()
  })

  test('MCP server detail page shows draft status', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText('draft')).toBeVisible()
    await expect(page.getByText('private')).toBeVisible()
  })

  test('edit MCP server metadata', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    await page.click('button:has-text("Edit")')
    await page.fill('input[name="name"]', `${MCP_NAME} edited`)
    await page.fill('input[name="license"]', 'MIT')
    await page.click('button:has-text("Save changes")')

    await page.waitForLoadState('networkidle')
    await expect(page.getByText(`${MCP_NAME} edited`)).toBeVisible()
    await expect(page.getByText('MIT')).toBeVisible()
  })

  test('toggle MCP server visibility to public', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    await page.click('button:has-text("Make public")')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('public')).toBeVisible()
  })

  test('toggle MCP server visibility back to private', async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await page.click('button:has-text("Make private")')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('private')).toBeVisible()
  })

  test('publish a version via API, then deprecate via UI', async ({ page }) => {
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

    // Navigate to admin detail — Deprecate button should appear.
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText('published')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Deprecate' })).toBeVisible()

    // Deprecate via UI.
    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Deprecate")')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('deprecated')).toBeVisible()
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

    await page.fill('input[name="namespace"]', PUBLISHER_SLUG)
    await page.fill('input[name="slug"]', AGENT_SLUG)
    await page.fill('input[name="name"]', AGENT_NAME)
    await page.fill('textarea[name="description"], input[name="description"]', 'An E2E test agent.')
    await page.click('button[type="submit"]')

    await page.waitForURL(/\/admin\/agents/)
    await expect(page.getByText(AGENT_NAME)).toBeVisible()
  })

  test('agent detail page shows draft status', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByText('draft')).toBeVisible()
    await expect(page.getByText('private')).toBeVisible()
  })

  test('edit agent metadata', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)

    await page.click('button:has-text("Edit")')
    await page.fill('input[name="name"]', `${AGENT_NAME} edited`)
    await page.click('button:has-text("Save changes")')

    await page.waitForLoadState('networkidle')
    await expect(page.getByText(`${AGENT_NAME} edited`)).toBeVisible()
  })

  test('toggle agent visibility to public', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await page.click('button:has-text("Make public")')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('public')).toBeVisible()
  })

  test('delete an agent', async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)

    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Delete")')

    await page.waitForURL(/\/admin\/agents$/)
    await expect(page.getByText(AGENT_NAME)).not.toBeVisible()
  })
})

// ── Publisher delete (must be last — publisher owns nothing after server/agent deleted) ──

test.describe('Admin: Publisher delete', () => {
  test('delete a publisher once its entries are gone', async ({ page }) => {
    await goTo(page, `/admin/publishers/${PUBLISHER_SLUG}`)

    page.on('dialog', dialog => dialog.accept())
    await page.click('button:has-text("Delete")')

    await page.waitForURL(/\/admin\/publishers$/)
    await expect(page.getByText(PUBLISHER_NAME)).not.toBeVisible()
  })
})
