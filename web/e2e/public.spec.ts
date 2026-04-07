/**
 * public.spec.ts
 *
 * End-to-end tests for the public (unauthenticated) UI and visibility
 * enforcement:
 *
 *  1. Homepage renders without errors.
 *  2. MCP listing is accessible without auth.
 *  3. Agent listing is accessible without auth.
 *  4. Private entries are NOT visible in the public listing.
 *  5. Admin routes redirect unauthenticated visitors to the sign-in page.
 */

import { test, expect } from "@playwright/test"

test.describe("Public: Homepage", () => {
  test("renders successfully", async ({ page }) => {
    await page.goto("/")
    await expect(page).toHaveTitle(/AI Registry|registry/i)
    // No error banners.
    await expect(page.getByText(/500|Internal Server Error/i)).not.toBeVisible()
  })
})

test.describe("Public: MCP Servers listing", () => {
  test("page loads and shows server list or empty state", async ({ page }) => {
    await page.goto("/mcp")
    await page.waitForLoadState("networkidle")
    // Either a list item or an explicit empty state should be present.
    const hasItems = await page.locator("ul li, [data-testid='mcp-item'], table tbody tr").count()
    const hasEmpty = await page.locator("text=/no servers|no results|empty/i").count()
    expect(hasItems + hasEmpty).toBeGreaterThan(0)
  })

  test("private servers do not appear in the public listing", async ({
    page,
  }) => {
    // private entries have visibility='private' — the backend filters them out
    // for unauthenticated requests.  We check the page text doesn't contain
    // the magic marker we'd expect only on a private fixture.
    await page.goto("/mcp")
    await page.waitForLoadState("networkidle")
    // The text "private" as a visibility badge should NOT appear on a public page.
    const privateLabels = await page.locator("text=private").count()
    expect(privateLabels).toBe(0)
  })
})

test.describe("Public: Agents listing", () => {
  test("page loads and shows agent list or empty state", async ({ page }) => {
    await page.goto("/agents")
    await page.waitForLoadState("networkidle")
    const hasItems = await page.locator("ul li, [data-testid='agent-item'], table tbody tr").count()
    const hasEmpty = await page.locator("text=/no agents|no results|empty/i").count()
    expect(hasItems + hasEmpty).toBeGreaterThan(0)
  })
})

test.describe("Public: Auth enforcement on admin routes", () => {
  test("GET /admin redirects unauthenticated visitors", async ({ page }) => {
    const response = await page.goto("/admin")
    const finalUrl = page.url()
    // Should redirect to sign-in page (Auth.js or Keycloak).
    const redirectedToAuth =
      finalUrl.includes("/api/auth/signin") ||
      finalUrl.includes("/realms/") ||
      finalUrl.includes("login") ||
      // Or Next.js middleware returned a 307 that ultimately landed elsewhere.
      (response?.status() ?? 200) === 307
    expect(redirectedToAuth || finalUrl !== "http://localhost:3000/admin").toBeTruthy()
  })

  test("GET /admin/mcp redirects unauthenticated visitors", async ({ page }) => {
    await page.goto("/admin/mcp")
    const finalUrl = page.url()
    expect(finalUrl).not.toMatch(/^http:\/\/localhost:3000\/admin\/mcp$/)
  })

  test("GET /admin/agents redirects unauthenticated visitors", async ({
    page,
  }) => {
    await page.goto("/admin/agents")
    const finalUrl = page.url()
    expect(finalUrl).not.toMatch(/^http:\/\/localhost:3000\/admin\/agents$/)
  })
})
