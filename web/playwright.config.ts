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
  reporter: process.env.CI ? "github" : "list",

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
      testMatch: /admin\.spec\.ts/,
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
      testMatch: /public\.spec\.ts/,
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
  ],
})
