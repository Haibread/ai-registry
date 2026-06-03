/**
 * global.setup.ts
 *
 * Authenticates each fixture user against Keycloak via the OIDC PKCE flow and
 * saves their storage state under e2e/.auth/<role>.json. The brokered callback
 * redirects back to the SPA with a one-time handoff code in the URL fragment,
 * which the SPA exchanges for a registry token pair; the refresh token lands in
 * localStorage, which storageState captures. Specs reuse these states via
 * `storageState` so they never have to log in again — on load each spec's SPA
 * refreshes the stored token into an in-memory access token.
 *
 * The four fixture users mirror the dev realm
 * (deploy/keycloak-realm-dev.json):
 *
 *   - admin     — realm role `admin` (every write path)
 *   - author    — groups `anthropic-core`, `anthropic-labs` (Editor
 *                 authoring + submit-for-review via a group role grant)
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
  // email+password form. The e2e identities live in Keycloak, so
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

    // The brokered OIDC callback redirects back to the app with a one-time
    // handoff code in the fragment; the SPA exchanges it for a token pair. For
    // the no-roles `user` the sign-in still succeeds — the 403s arrive later, at
    // the API layer when the test attempts a write. Wait for the redirect away
    // from Keycloak, then for the SPA to persist the refresh token.
    await page.waitForURL(url => !/\/realms\//.test(url.toString()), {
      timeout: 30_000,
    })
    await page.waitForLoadState('networkidle')
    await page.waitForFunction(() => !!window.localStorage.getItem('ai_registry_access'), null, {
      timeout: 30_000,
    })

    const file = path.join(__dirname, `.auth/${fx.role}.json`)
    await page.context().storageState({ path: file })
  })
}
