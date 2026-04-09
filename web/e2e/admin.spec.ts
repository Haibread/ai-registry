/**
 * admin.spec.ts
 *
 * End-to-end tests for the admin UI flows:
 *
 *  1. Publisher CRUD — create a publisher, verify it appears in the list.
 *  2. MCP Server CRUD — create an MCP server under the publisher.
 *  3. Agent CRUD — create an agent under the publisher.
 *  4. Visibility toggle — make an MCP server public, verify badge changes.
 *  5. Deprecate flow — publish a version, deprecate the server, verify status.
 *
 * These tests run sequentially (workers: 1) because they share mutable DB
 * state. The setup project must run first to populate e2e/.auth/admin.json.
 */

import { test, expect, type Page } from "@playwright/test"

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
  await page.waitForLoadState("networkidle")
}

// ── tests ─────────────────────────────────────────────────────────────────────

test.describe("Admin: Publisher CRUD", () => {
  test("create a publisher", async ({ page }) => {
    await goTo(page, "/admin/publishers/new")

    await page.fill('input[name="slug"]', PUBLISHER_SLUG)
    await page.fill('input[name="name"]', PUBLISHER_NAME)
    await page.click('button[type="submit"]')

    // Redirected to the publishers list (or detail) after creation.
    await page.waitForURL(/\/admin\/publishers/)
    await expect(page.getByText(PUBLISHER_NAME)).toBeVisible()
  })
})

test.describe("Admin: MCP Server CRUD", () => {
  test("create an MCP server", async ({ page }) => {
    await goTo(page, "/admin/mcp/new")

    await page.fill('input[name="namespace"]', PUBLISHER_SLUG)
    await page.fill('input[name="slug"]', MCP_SLUG)
    await page.fill('input[name="name"]', MCP_NAME)
    await page.fill('textarea[name="description"], input[name="description"]', "An E2E test MCP server.")
    await page.click('button[type="submit"]')

    await page.waitForURL(/\/admin\/mcp/)
    await expect(page.getByText(MCP_NAME)).toBeVisible()
  })

  test("MCP server detail page shows draft status", async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText("draft")).toBeVisible()
    await expect(page.getByText("private")).toBeVisible()
  })

  test("toggle MCP server visibility to public", async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)

    // Click "Make public" button.
    await page.click('button:has-text("Make public")')

    // After form submit + redirect, badge should read "public".
    await page.waitForURL(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText("public")).toBeVisible()
  })

  test("toggle MCP server visibility back to private", async ({ page }) => {
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await page.click('button:has-text("Make private")')
    await page.waitForURL(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText("private")).toBeVisible()
  })
})

test.describe("Admin: Agent CRUD", () => {
  test("create an agent", async ({ page }) => {
    await goTo(page, "/admin/agents/new")

    await page.fill('input[name="namespace"]', PUBLISHER_SLUG)
    await page.fill('input[name="slug"]', AGENT_SLUG)
    await page.fill('input[name="name"]', AGENT_NAME)
    await page.fill('textarea[name="description"], input[name="description"]', "An E2E test agent.")
    await page.click('button[type="submit"]')

    await page.waitForURL(/\/admin\/agents/)
    await expect(page.getByText(AGENT_NAME)).toBeVisible()
  })

  test("agent detail page shows draft status", async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByText("draft")).toBeVisible()
    await expect(page.getByText("private")).toBeVisible()
  })

  test("toggle agent visibility to public", async ({ page }) => {
    await goTo(page, `/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await page.click('button:has-text("Make public")')
    await page.waitForURL(`/admin/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByText("public")).toBeVisible()
  })
})

test.describe("Admin: Deprecate flow (via API — MCP server)", () => {
  /**
   * The UI deprecate button is only rendered for published servers.
   * We use the server API directly to create + publish a version first,
   * then verify the UI deprecate button works.
   *
   * API calls go through the proxy route handler which injects the session
   * token from the authenticated browser context.
   */

  test("publish a version via API proxy, then deprecate via UI", async ({
    page,
  }) => {
    // Step 1: create a version via the proxy.
    const createVersionRes = await page.request.post(
      `/api/proxy/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`,
      {
        data: {
          version: "1.0.0",
          runtime: "stdio",
          protocol_version: "2025-03-26",
          packages: [
            {
              registryType: "npm",
              identifier: "@e2e/test-server",
              version: "1.0.0",
              transport: { type: "stdio" },
            },
          ],
        },
      }
    )
    expect(createVersionRes.ok()).toBeTruthy()

    // Step 2: publish it.
    const publishRes = await page.request.post(
      `/api/proxy/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`
    )
    expect(publishRes.ok()).toBeTruthy()

    // Step 3: navigate to admin detail — the Deprecate button should appear.
    await goTo(page, `/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText("published")).toBeVisible()
    await expect(page.getByRole("button", { name: "Deprecate" })).toBeVisible()

    // Step 4: click Deprecate.
    await page.click('button:has-text("Deprecate")')
    await page.waitForURL(`/admin/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByText("deprecated")).toBeVisible()
  })
})
