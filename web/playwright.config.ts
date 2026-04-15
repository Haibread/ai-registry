import { defineConfig, devices } from "@playwright/test"

/**
 * Playwright configuration for AI Registry E2E tests.
 *
 * Prerequisites:
 *   - The full docker-compose stack must be running (web + server + keycloak + postgres).
 *   - A test admin user must exist in Keycloak (see E2E_ADMIN_* env vars below).
 *
 * Run:
 *   npm run test:e2e              # headless
 *   npm run test:e2e:ui           # interactive UI mode
 */

const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3000"

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false, // admin tests mutate state; keep sequential
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  // In CI: emit both GitHub annotations (inline in the run log) AND an HTML
  // report so the `upload-artifact` step in .github/workflows/e2e.yml has
  // something to publish. `open: 'never'` stops Playwright from trying to
  // launch a browser on the headless runner.
  reporter: process.env.CI
    ? [['github'], ['html', { open: 'never', outputFolder: 'playwright-report' }]]
    : 'list',

  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },

  projects: [
    // Setup project: authenticate once and save storage state.
    {
      name: "setup",
      testMatch: /global\.setup\.ts/,
    },
    {
      name: "admin-chromium",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      // Anchor on a path separator so this does not also match
      // coverage-admin.spec.ts (which is owned by the coverage-admin project).
      testMatch: /(^|\/)admin\.spec\.ts$/,
    },
    {
      name: "admin-stats",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /admin-stats\.spec\.ts/,
    },
    {
      name: "public-chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
      testMatch: /(^|\/)public\.spec\.ts$/,
    },
    // Detail-page tests seed their own data via the admin API, so they need
    // the authenticated storage state. The page navigations themselves target
    // public routes — auth does not alter their rendered content.
    {
      name: "detail-chromium",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /detail\.spec\.ts/,
    },
    // Admin-side coverage gaps (search/filter, bulk actions, UI publish,
    // error states). Uses the admin storageState and mutates DB state, so
    // it runs serially after setup.
    {
      name: "coverage-admin",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /coverage-admin\.spec\.ts/,
    },
    // Public-side coverage gaps (publisher detail, theme toggle, public
    // search, private/missing 404). Seeds via the admin API but navigates
    // as a public visitor.
    {
      name: "coverage-public",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /coverage-public\.spec\.ts/,
    },
  ],
})
