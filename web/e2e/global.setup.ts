/**
 * global.setup.ts
 *
 * Authenticates as an admin user via the Keycloak-backed OIDC flow and saves
 * the browser storage state to e2e/.auth/admin.json. All admin tests reuse
 * this saved state to avoid logging in before every test.
 *
 * Required env vars:
 *   E2E_ADMIN_EMAIL    - admin user email in Keycloak (default: admin@example.com)
 *   E2E_ADMIN_PASSWORD - admin user password         (default: admin)
 */

import { test as setup, expect } from "@playwright/test"
import path from "path"

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin@example.com"
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "admin"
const AUTH_FILE = path.join(__dirname, ".auth/admin.json")

setup("authenticate as admin", async ({ page }) => {
  // Navigate to a protected admin page to trigger the OIDC redirect.
  await page.goto("/admin")

  // Auth.js redirects to Keycloak's login page.
  // Wait for the Keycloak username input to appear.
  await page.waitForURL(/\/realms\/ai-registry\/protocol\/openid-connect\/auth/)
  await expect(page.locator("#username, input[name='username']")).toBeVisible()

  await page.fill("#username, input[name='username']", ADMIN_EMAIL)
  await page.fill("#password, input[name='password']", ADMIN_PASSWORD)
  await page.click("#kc-login, input[type='submit']")

  // After successful login we should land on the admin dashboard.
  await page.waitForURL(/\/admin/)
  await expect(page.locator("h1, [data-testid='admin-heading']")).toBeVisible()

  // Persist the authenticated session for all admin tests.
  await page.context().storageState({ path: AUTH_FILE })
})
