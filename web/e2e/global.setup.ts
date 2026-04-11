/**
 * global.setup.ts
 *
 * Authenticates as an admin user via the Keycloak-backed OIDC flow and saves
 * the browser storage state to e2e/.auth/admin.json. All admin tests reuse
 * this saved state to avoid logging in before every test.
 *
 * Flow:
 *   1. Navigate to the homepage (RequireAuth redirects /admin → / for guests).
 *   2. Click the "Sign in" button → initiates the OIDC redirect to Keycloak.
 *   3. Fill Keycloak credentials.
 *   4. AuthCallback exchanges the code and navigates to /admin.
 *   5. Save storageState (oidc-client-ts persists the Bearer token in localStorage).
 *
 * Required env vars:
 *   E2E_ADMIN_EMAIL    - admin user email in Keycloak (default: admin@example.com)
 *   E2E_ADMIN_PASSWORD - admin user password         (default: admin)
 */

import { test as setup, expect } from '@playwright/test'
import path from 'path'

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? 'admin@example.com'
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'admin'
const AUTH_FILE = path.join(__dirname, '.auth/admin.json')

setup('authenticate as admin', async ({ page }) => {
  // Start from the homepage — unauthenticated visits to /admin are redirected
  // here by RequireAuth (<Navigate to="/" />).
  await page.goto('/')
  await page.waitForLoadState('networkidle')

  // Click the Sign In button to initiate the OIDC Authorization Code + PKCE flow.
  await page.click('button:has-text("Sign in")')

  // Keycloak login page.
  await page.waitForURL(/\/realms\/ai-registry\/protocol\/openid-connect\/auth/)
  await expect(page.locator('#username, input[name="username"]')).toBeVisible()

  await page.fill('#username, input[name="username"]', ADMIN_EMAIL)
  await page.fill('#password, input[name="password"]', ADMIN_PASSWORD)
  await page.click('#kc-login, input[type="submit"]')

  // AuthCallback (at /auth/callback) exchanges the code then navigates to /admin.
  await page.waitForURL(/\/admin/, { timeout: 30_000 })
  await expect(page.locator('h1')).toBeVisible()

  // Persist the authenticated session (localStorage with the OIDC tokens).
  await page.context().storageState({ path: AUTH_FILE })
})
