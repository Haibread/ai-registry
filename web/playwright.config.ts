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
    // Per-entry activity feed + admin /audit page smoke tests. Seeds via the
    // admin API (which generates real audit rows) then inspects both the
    // privacy-scrubbed public feed and the full-fidelity admin view.
    {
      name: "activity",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /activity\.spec\.ts/,
    },
    // Change-approval workflow: submit / approve / reject / withdraw,
    // discriminated 409 responses, request + approve deletion — drives the
    // live API surface directly, no UI.
    {
      name: "change-approval",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /change-approval\.spec\.ts/,
    },
    // Entry-change review queue: visibility / deprecate / metadata edits route
    // through the queue for Editors (202 → approve) while Server Admins keep the
    // immediate path. Switches identity per-step (admin + author) via
    // browser.newContext, so no project-level storageState — like phase7-flows.
    {
      name: "entry-change",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /entry-change\.spec\.ts/,
    },
    // Review-queue / version-section / request-deletion UI flows.
    // Drives the actual browser DOM (the change-approval project goes
    // through the API directly), so this catches handler/UI contract
    // drift that mocked unit tests miss.
    {
      name: "review-ui",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /review-ui\.spec\.ts/,
    },
    // RBAC management UI: drives the Groups / Users / Grants admin pages
    // through the browser DOM (the phase7-flows project sets grants up via
    // the API), catching handler/UI contract drift the mocked unit tests
    // can't. Runs as the admin Server-Admin session.
    {
      name: "rbac-ui",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /rbac-ui\.spec\.ts/,
    },
    // Local email + password login. Authenticates through the
    // /login form itself rather than a stored OIDC session, so it has no
    // `setup` dependency and no storageState. Requires the stack booted with
    // AUTH_LOCAL_LOGIN_ENABLED=true + a seeded bootstrap admin (CI wires this
    // in docker-compose.ci.yml + the e2e workflow).
    {
      name: "local-login",
      use: {
        ...devices["Desktop Chrome"],
      },
      testMatch: /local-login\.spec\.ts/,
    },
    // Publisher-scoped authorization: proves that author@ /
    // reviewer@ / user@ fixture tokens actually carry the right
    // `groups[]` claim and that RequirePublisherRole +
    // RequireReviewer enforce the grants end-to-end. Each test
    // switches storage state per-describe block (no project-level
    // default — the spec is intentionally polyglot).
    {
      name: "phase7-flows",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /phase7-flows\.spec\.ts/,
    },
    // Publisher-scoped admin home: the switcher, the scoped
    // Overview, and the Members + Activity pages. Runs as the Server-Admin
    // session, which can scope to any publisher, and seeds its own data.
    {
      name: "publisher-home",
      use: {
        ...devices["Desktop Chrome"],
        storageState: "e2e/.auth/admin.json",
      },
      dependencies: ["setup"],
      testMatch: /publisher-home\.spec\.ts/,
    },
    // User-journey scenarios: full create/edit/patch/delete lifecycles plus
    // the switcher-scoping, create-form pre-select, "Load more" pagination,
    // rich version fields, read-only settings, and bulk-confirm UX. Switches
    // identity per-test (admin + author) via browser.newContext, so it has no
    // project-level storageState — like phase7-flows.
    {
      name: "user-journeys",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /user-journeys\.spec\.ts/,
    },
    // Failure-path + permission-gating coverage. Uses request interception to
    // force deterministic API 500s so the UI error handling is exercised, and
    // drives the author (editor) session for role gating. No project-level
    // storageState — identity is switched per-test via browser.newContext.
    {
      name: "ux-edge-cases",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /ux-edge-cases\.spec\.ts/,
    },
    // Admin UX review follow-ups (linked list names + sort, dirty-form guard,
    // detail read view / not-found branch, report drill-down). Identity is
    // switched per-test via browser.newContext, like ux-edge-cases.
    {
      name: "admin-ux-polish",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /admin-ux-polish\.spec\.ts/,
    },
    {
      name: "user-access",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /user-access\.spec\.ts/,
    },
    {
      name: "request-public",
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
      testMatch: /request-public\.spec\.ts/,
    },
  ],
})
