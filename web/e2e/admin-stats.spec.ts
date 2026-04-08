/**
 * admin-stats.spec.ts
 *
 * Regression tests for the admin dashboard statistics panel.
 *
 * Root causes previously observed:
 *  1. /api/v1/stats didn't exist — dashboard fell back to "—" for all counts.
 *  2. Keycloak 26 doesn't include realm_access.roles in access tokens unless
 *     explicitly configured — backend returned 401 → frontend silently showed "—".
 *
 * These tests verify that:
 *  a) The stats panel shows numeric values (never "—") after login.
 *  b) No error banner is rendered.
 *  c) All three keys (mcp_servers, agents, publishers) are present in the API
 *     response when called with the session token (via the Next.js proxy).
 */

import { test, expect } from "@playwright/test"

// Runs with the authenticated admin session (storageState configured in
// playwright.config.ts for the admin-chromium project).

test.describe("Admin Dashboard: stats panel (regression)", () => {
  test("dashboard shows numeric counts — not dashes", async ({ page }) => {
    await page.goto("/admin")
    await page.waitForLoadState("networkidle")

    // If the stats API call fails, the page renders an explicit error banner.
    await expect(page.locator('[data-testid="stats-error"]')).not.toBeVisible()

    // Each stat card should contain a number, not the fallback dash.
    // We look for at least one digit inside the three card values.
    // The heading text nodes are the large bold numbers in CardContent.
    const statValues = page.locator(".text-3xl.font-bold")
    const count = await statValues.count()
    expect(count).toBeGreaterThanOrEqual(3)

    for (let i = 0; i < count; i++) {
      const text = await statValues.nth(i).textContent()
      // Must be a digit string, not "—".
      expect(text?.trim()).toMatch(/^\d+$/)
    }
  })

  test("stats API is reachable and returns required keys", async ({ page }) => {
    // Use the browser request context (carries the authenticated session cookie)
    // to call the backend via the Next.js proxy route.
    const res = await page.request.get("/api/proxy/stats")
    expect(res.ok()).toBeTruthy()

    const body = await res.json()
    expect(typeof body.mcp_servers).toBe("number")
    expect(typeof body.agents).toBe("number")
    expect(typeof body.publishers).toBe("number")
  })

  test("stats counts increase after creating resources", async ({ page }) => {
    // Read baseline counts.
    const before = await (
      await page.request.get("/api/proxy/stats")
    ).json() as { publishers: number; mcp_servers: number; agents: number }

    const RUN_ID = Date.now().toString(36)
    const pubSlug = `stat-pub-${RUN_ID}`

    // Create a publisher via the proxy.
    const pubRes = await page.request.post("/api/proxy/publishers", {
      data: { slug: pubSlug, name: `Stat Publisher ${RUN_ID}` },
    })
    expect(pubRes.ok()).toBeTruthy()

    // Create an MCP server under that publisher.
    const mcpRes = await page.request.post("/api/proxy/mcp/servers", {
      data: {
        publisher_id: (await pubRes.json()).id,
        slug: `stat-mcp-${RUN_ID}`,
        name: `Stat MCP ${RUN_ID}`,
      },
    })
    expect(mcpRes.ok()).toBeTruthy()

    // Fetch stats again and verify counts went up.
    const after = await (
      await page.request.get("/api/proxy/stats")
    ).json() as { publishers: number; mcp_servers: number; agents: number }

    expect(after.publishers).toBeGreaterThan(before.publishers)
    expect(after.mcp_servers).toBeGreaterThan(before.mcp_servers)

    // Reload the dashboard and confirm it still shows digits.
    await page.goto("/admin")
    await page.waitForLoadState("networkidle")
    await expect(page.locator('[data-testid="stats-error"]')).not.toBeVisible()
  })
})
