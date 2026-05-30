/**
 * local-login.spec.ts
 *
 * End-to-end test for the local email + password front door (ADR 0006):
 * the registry's own login path that does not involve the OIDC IdP. Drives
 * the /login form against the live stack, where the server is booted with
 * AUTH_LOCAL_LOGIN_ENABLED=true, an ephemeral signing key, and a seeded
 * bootstrap Server Admin (see docker-compose.ci.yml + .github/workflows/e2e.yml).
 *
 * Unlike the other admin specs this one does NOT use a stored OIDC session —
 * it authenticates through the local form itself, which is the whole point.
 *
 * Credentials come from env (matching the bootstrap admin the server seeds);
 * the defaults mirror the CI workflow so a correctly-configured local stack
 * can run it too.
 *
 * Run:  npm run test:e2e -- --project=local-login
 */

import { test, expect } from '@playwright/test'

const ADMIN_EMAIL = process.env.E2E_LOCAL_ADMIN_EMAIL ?? 'local-admin@example.com'
const ADMIN_PASSWORD = process.env.E2E_LOCAL_ADMIN_PASSWORD ?? 'e2e-local-admin-pass'

test.describe('Local email + password login', () => {
  test('signs in via the local form and reaches an authenticated admin page', async ({ page }) => {
    await page.goto('/login')

    // The form is always rendered; the local submit button is exactly
    // "Sign in" (the OIDC button is "Sign in with your organization").
    await page.fill('#email', ADMIN_EMAIL)
    await page.fill('#password', ADMIN_PASSWORD)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()

    // loginLocal stores the registry token and navigates to /admin.
    await page.waitForURL(/\/admin(?:$|\/|\?)/, { timeout: 20_000 })

    // Prove the registry token actually authorizes an API-backed admin page:
    // /admin/users fetches /api/v1/users. A rejected token would trip the
    // 401 middleware → clearSession → redirect back to /login.
    await page.goto('/admin/users')
    await expect(page.getByRole('heading', { name: 'Users', exact: true })).toBeVisible({ timeout: 15_000 })
    await expect(page).toHaveURL(/\/admin\/users/)
  })

  test('rejects a wrong password and stays on the login page', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#email', ADMIN_EMAIL)
    await page.fill('#password', 'definitely-the-wrong-password')
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()

    // The form surfaces the error and does not navigate away.
    await expect(page.getByRole('alert')).toBeVisible({ timeout: 15_000 })
    await expect(page).toHaveURL(/\/login/)
  })
})
