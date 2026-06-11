/**
 * tags.spec.ts
 *
 * Instance-tag vocabulary end-to-end flow:
 *
 *  1. Server Admin curates the vocabulary on /admin/tags (create, deactivate,
 *     activate, delete).
 *  2. A publisher ticks a tag on a new version; after publish the tag shows
 *     as a chip on the public detail page and the catalog `?tag=` filter
 *     matches the entry.
 *  3. The API rejects unknown tags on version create (422) and refuses to
 *     hard-delete an in-use tag (409).
 *
 * Runs as the Server-Admin session (storageState) and mutates DB state, so
 * it stays in the sequential worker like the other admin suites.
 */

import { test, expect, type Page } from '@playwright/test'
import { apiDelete, apiPost, confirmDialog } from './helpers'

// Unique suffix to avoid collisions between test runs.
const RUN_ID = Date.now().toString(36)
const TAG_SLUG = `e2e-tag-${RUN_ID}`
const TAG_NAME = `E2E Tag ${RUN_ID}`
const UI_TAG_SLUG = `e2e-uitag-${RUN_ID}`
const UI_TAG_NAME = `E2E UI Tag ${RUN_ID}`
const PUBLISHER_SLUG = `e2e-tagpub-${RUN_ID}`
const MCP_SLUG = `e2e-tagsrv-${RUN_ID}`

async function goTo(page: Page, path: string) {
  await page.goto(path)
  await page.waitForLoadState('domcontentloaded')
}

test.describe('Instance tags: admin curation UI', () => {
  test('create, deactivate, activate, delete a tag on /admin/tags', async ({ page }) => {
    await goTo(page, '/admin/tags')

    // Create through the form.
    await page.fill('input[name="slug"]', UI_TAG_SLUG)
    await page.fill('input[name="name"]', UI_TAG_NAME)
    await page.fill('input[name="description"]', 'Created by the e2e suite')
    await page.selectOption('select[name="color"]', 'green')
    await page.getByRole('button', { name: /add tag/i }).click()

    const row = page.getByRole('row', { name: new RegExp(UI_TAG_NAME) })
    await expect(row).toBeVisible()
    await expect(row.getByText('Active')).toBeVisible()

    // Deactivate, then reactivate.
    await row.getByRole('button', { name: 'Deactivate' }).click()
    await expect(row.getByText('Deactivated')).toBeVisible()
    await row.getByRole('button', { name: 'Activate' }).click()
    await expect(row.getByText('Active')).toBeVisible()

    // Delete (unused, so the API allows it).
    await row.getByRole('button', { name: 'Delete' }).click()
    await confirmDialog(page, 'Delete')
    await expect(page.getByRole('row', { name: new RegExp(UI_TAG_NAME) })).toHaveCount(0)
  })
})

test.describe('Instance tags: publisher tick → public display + filter', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage({ storageState: 'e2e/.auth/admin.json' })

    let res = await apiPost(page, '/api/v1/tags', {
      slug: TAG_SLUG,
      name: TAG_NAME,
      description: 'Ticked by the e2e suite',
      color: 'teal',
    })
    expect(res.status(), await res.text()).toBe(201)

    res = await apiPost(page, '/api/v1/publishers', {
      slug: PUBLISHER_SLUG,
      name: `E2E Tag Publisher ${RUN_ID}`,
    })
    expect(res.status(), await res.text()).toBe(201)

    res = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUBLISHER_SLUG,
      slug: MCP_SLUG,
      name: `E2E Tagged Server ${RUN_ID}`,
      description: 'Server carrying an instance tag',
    })
    expect(res.status(), await res.text()).toBe(201)

    // The publisher ticks the tag on the version; publish freezes it.
    res = await apiPost(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`, {
      version: '1.0.0',
      runtime: 'stdio',
      protocol_version: '2025-03-26',
      tags: [TAG_SLUG],
    })
    expect(res.status(), await res.text()).toBe(201)

    res = await apiPost(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`, {})
    expect(res.status(), await res.text()).toBe(200)

    res = await apiPost(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/visibility`, {
      visibility: 'public',
    })
    expect(res.status(), await res.text()).toBe(200)

    await page.close()
  })

  test('the ticked tag renders as a chip on the public detail page', async ({ page }) => {
    await goTo(page, `/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText(TAG_NAME).first()).toBeVisible()
  })

  test('the catalog tag filter matches the entry', async ({ page }) => {
    await goTo(page, `/mcp?tag=${TAG_SLUG}`)
    await expect(page.getByText(`E2E Tagged Server ${RUN_ID}`)).toBeVisible()
  })

  test('unknown tags are rejected on version create (422)', async ({ page }) => {
    const res = await apiPost(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`, {
      version: '2.0.0',
      runtime: 'stdio',
      protocol_version: '2025-03-26',
      tags: [`${TAG_SLUG}-does-not-exist`],
    })
    expect(res.status()).toBe(422)
    expect(await res.text()).toContain(`${TAG_SLUG}-does-not-exist`)
  })

  test('an in-use tag cannot be hard-deleted (409) but can be deactivated', async ({ page }) => {
    const del = await apiDelete(page, `/api/v1/tags/${TAG_SLUG}`)
    expect(del.status()).toBe(409)
    expect(await del.text()).toContain('deactivate')

    await goTo(page, '/admin/tags')
    const row = page.getByRole('row', { name: new RegExp(TAG_NAME) })
    await row.getByRole('button', { name: 'Deactivate' }).click()
    await expect(row.getByText('Deactivated')).toBeVisible()

    // The frozen version keeps displaying the (deactivated) tag publicly.
    await goTo(page, `/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText(TAG_NAME).first()).toBeVisible()
  })
})
