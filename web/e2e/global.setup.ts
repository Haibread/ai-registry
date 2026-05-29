/**
 * global.setup.ts
 *
 * Authenticates each fixture user against Keycloak via the OIDC PKCE flow
 * and saves their storage state under e2e/.auth/<role>.json. Specs reuse
 * these states via `storageState` in playwright.config.ts (or per-describe
 * with `test.use({ storageState })`) so they never have to log in again.
 *
 * The four fixture users mirror the dev realm
 * (deploy/keycloak-realm-dev.json):
 *
 *   - admin     — realm role `admin` (every write path)
 *   - author    — groups `anthropic-core`, `anthropic-labs` (workspace
 *                 authoring + submit-for-review)
 *   - reviewer  — group `registry-reviewers` (approve / reject)
 *   - user      — no roles, no groups (403 baseline)
 *
 * Each user's credentials can be overridden via E2E_<ROLE>_EMAIL and
 * E2E_<ROLE>_PASSWORD env vars; the defaults match the dev realm so
 * `npm run test:e2e` works out of the box.
 */

import { test as setup, expect, type Page } from '@playwright/test'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

type Fixture = {
  role: 'admin' | 'author' | 'reviewer' | 'user'
  email: string
  password: string
}

const fixtures: Fixture[] = [
  {
    role: 'admin',
    email: process.env.E2E_ADMIN_EMAIL ?? 'admin@example.com',
    password: process.env.E2E_ADMIN_PASSWORD ?? 'admin',
  },
  {
    role: 'author',
    email: process.env.E2E_AUTHOR_EMAIL ?? 'author@example.com',
    password: process.env.E2E_AUTHOR_PASSWORD ?? 'author',
  },
  {
    role: 'reviewer',
    email: process.env.E2E_REVIEWER_EMAIL ?? 'reviewer@example.com',
    password: process.env.E2E_REVIEWER_PASSWORD ?? 'reviewer',
  },
  {
    role: 'user',
    email: process.env.E2E_USER_EMAIL ?? 'user@example.com',
    password: process.env.E2E_USER_PASSWORD ?? 'user',
  },
]

async function loginAs(page: Page, email: string, password: string) {
  // The login page offers both the OIDC ("organization") button and a local
  // email+password form (ADR 0006). The e2e identities live in Keycloak, so
  // drive the OIDC button to initiate the redirect.
  await page.goto('/login')
  await page.waitForLoadState('networkidle')

  await page.click('button:has-text("Sign in with your organization")')

  await page.waitForURL(/\/realms\/ai-registry\/protocol\/openid-connect\/auth/)
  await expect(page.locator('#username, input[name="username"]')).toBeVisible()

  await page.fill('#username, input[name="username"]', email)
  await page.fill('#password, input[name="password"]', password)
  await page.click('#kc-login, input[type="submit"]')
}

for (const fx of fixtures) {
  setup(`authenticate as ${fx.role}`, async ({ page }) => {
    await loginAs(page, fx.email, fx.password)

    // For admin / author / reviewer the AuthCallback lands on /admin (or
    // wherever RequireAuth would have sent them). For the no-roles `user`
    // the SPA still completes the OIDC exchange — the 403s arrive later,
    // at the API layer when the test attempts a write. Either way wait
    // for the callback to settle.
    await page.waitForURL(url => !/\/realms\//.test(url.toString()), {
      timeout: 30_000,
    })
    await page.waitForLoadState('networkidle')

    const file = path.join(__dirname, `.auth/${fx.role}.json`)
    await page.context().storageState({ path: file })
  })
}
