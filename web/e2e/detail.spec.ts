/**
 * detail.spec.ts
 *
 * End-to-end tests for the public MCP server and Agent detail pages.
 *
 * The admin CRUD suite in admin.spec.ts creates entries but never navigates
 * into their *public* detail pages, and public.spec.ts only covers listings.
 * These tests close that gap by asserting the v0.2 Connection card, tab
 * navigation, and JSON view all render against a real backend.
 *
 * Strategy:
 *   - Seed a publisher, an MCP server (with a remote package so the Connection
 *     & Runtime hero row renders), and an agent (with a published version) via
 *     the admin API using the shared storageState.
 *   - Publish and make both entries public.
 *   - Navigate as a page user and assert detail-page content.
 *   - Tear everything down in afterAll so the run is idempotent.
 *
 * Uses the admin storageState (injected via playwright.config.ts) so the tests
 * can authenticate API calls for seeding and teardown. The detail pages being
 * tested are public reads — authentication does not alter their content.
 */

import { test, expect } from '@playwright/test'
import { apiPost } from './helpers'

// Unique suffix to avoid collisions across runs.
const RUN_ID = Date.now().toString(36)
const PUBLISHER_SLUG = `e2e-detail-pub-${RUN_ID}`
const PUBLISHER_NAME = `E2E Detail Publisher ${RUN_ID}`
const MCP_SLUG = `e2e-detail-mcp-${RUN_ID}`
const MCP_NAME = `E2E Detail MCP ${RUN_ID}`
const AGENT_SLUG = `e2e-detail-agent-${RUN_ID}`
const AGENT_NAME = `E2E Detail Agent ${RUN_ID}`
const AGENT_ENDPOINT = 'https://agents.example.test/e2e-detail'

// Tests seed state once and read it; keep them serial so a test can assume the
// previous step completed successfully.
test.describe.configure({ mode: 'serial' })

async function apiDelete(page: import('@playwright/test').Page, path: string) {
  const token = await page.evaluate(() => {
    const key = Object.keys(localStorage).find(k => k.startsWith('oidc.user:'))
    if (!key) return ''
    try {
      return (JSON.parse(localStorage.getItem(key)!) as { access_token?: string }).access_token ?? ''
    } catch {
      return ''
    }
  })
  return page.request.delete(path, {
    headers: { Authorization: `Bearer ${token}` },
  })
}

test.describe('Public detail pages', () => {
  test.beforeAll(async ({ browser }) => {
    const context = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await context.newPage()

    // Navigate first so localStorage is accessible for apiPost.
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    // ── Publisher ────────────────────────────────────────────────────────
    const pubRes = await apiPost(page, '/api/v1/publishers', {
      slug: PUBLISHER_SLUG,
      name: PUBLISHER_NAME,
    })
    if (!pubRes.ok()) {
      throw new Error(`seed publisher failed: ${pubRes.status()} ${await pubRes.text()}`)
    }

    // ── MCP server with a remote package ─────────────────────────────────
    const mcpRes = await apiPost(page, '/api/v1/mcp/servers', {
      namespace: PUBLISHER_SLUG,
      slug: MCP_SLUG,
      name: MCP_NAME,
      description: 'An E2E detail page MCP server.',
    })
    if (!mcpRes.ok()) {
      throw new Error(`seed mcp server failed: ${mcpRes.status()} ${await mcpRes.text()}`)
    }

    const mcpVerRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions`,
      {
        version: '1.0.0',
        runtime: 'sse',
        protocol_version: '2025-03-26',
        packages: [
          {
            registryType: 'npm',
            identifier: '@e2e/detail-mcp',
            version: '1.0.0',
            transport: { type: 'sse', url: 'https://mcp.example.test/e2e-detail/sse' },
          },
        ],
      },
    )
    if (!mcpVerRes.ok()) {
      throw new Error(`seed mcp version failed: ${mcpVerRes.status()} ${await mcpVerRes.text()}`)
    }

    const mcpPubRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/versions/1.0.0/publish`,
      {},
    )
    if (!mcpPubRes.ok()) {
      throw new Error(`publish mcp version failed: ${mcpPubRes.status()} ${await mcpPubRes.text()}`)
    }

    const mcpVisRes = await apiPost(
      page,
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}/visibility`,
      { visibility: 'public' },
    )
    if (!mcpVisRes.ok()) {
      throw new Error(`make mcp public failed: ${mcpVisRes.status()} ${await mcpVisRes.text()}`)
    }

    // ── Agent ────────────────────────────────────────────────────────────
    const agentRes = await apiPost(page, '/api/v1/agents', {
      namespace: PUBLISHER_SLUG,
      slug: AGENT_SLUG,
      name: AGENT_NAME,
      description: 'An E2E detail page agent.',
    })
    if (!agentRes.ok()) {
      throw new Error(`seed agent failed: ${agentRes.status()} ${await agentRes.text()}`)
    }

    const agentVerRes = await apiPost(
      page,
      `/api/v1/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}/versions`,
      {
        version: '1.0.0',
        endpoint_url: AGENT_ENDPOINT,
        protocol_version: '0.3.0',
        default_input_modes: ['text/plain'],
        default_output_modes: ['text/plain'],
        skills: [
          {
            id: 'detail-skill',
            name: 'E2E Detail Skill',
            description: 'A dummy skill used for detail-page e2e assertions.',
            tags: ['e2e'],
          },
        ],
        authentication: [{ scheme: 'Bearer' }],
      },
    )
    if (!agentVerRes.ok()) {
      throw new Error(`seed agent version failed: ${agentVerRes.status()} ${await agentVerRes.text()}`)
    }

    const agentPubRes = await apiPost(
      page,
      `/api/v1/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}/versions/1.0.0/publish`,
      {},
    )
    if (!agentPubRes.ok()) {
      throw new Error(`publish agent version failed: ${agentPubRes.status()} ${await agentPubRes.text()}`)
    }

    const agentVisRes = await apiPost(
      page,
      `/api/v1/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}/visibility`,
      { visibility: 'public' },
    )
    if (!agentVisRes.ok()) {
      throw new Error(`make agent public failed: ${agentVisRes.status()} ${await agentVisRes.text()}`)
    }

    // Sanity check — the public API can read both back without a token.
    const anonMcp = await page.request.get(
      `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}`,
    )
    if (!anonMcp.ok()) {
      throw new Error(`anon read mcp failed: ${anonMcp.status()}`)
    }
    const anonAgent = await page.request.get(
      `/api/v1/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`,
    )
    if (!anonAgent.ok()) {
      throw new Error(`anon read agent failed: ${anonAgent.status()}`)
    }

    await context.close()
  })

  test.afterAll(async ({ browser }) => {
    const context = await browser.newContext({ storageState: 'e2e/.auth/admin.json' })
    const page = await context.newPage()
    await page.goto('/admin')
    await expect(page.locator('h1')).toBeVisible({ timeout: 15_000 })

    // Best-effort cleanup — don't fail teardown if something is already gone.
    await apiDelete(page, `/api/v1/mcp/servers/${PUBLISHER_SLUG}/${MCP_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`).catch(() => {})
    await apiDelete(page, `/api/v1/publishers/${PUBLISHER_SLUG}`).catch(() => {})

    await context.close()
  })

  // ── MCP detail page ───────────────────────────────────────────────────

  test('MCP detail page renders the name, identifier, and Connection card', async ({ page }) => {
    await page.goto(`/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByRole('heading', { name: MCP_NAME })).toBeVisible({ timeout: 15_000 })

    // The remote transport branch shows the "Connection & Runtime" header and
    // an Endpoint URL hero row linking to the package's transport URL.
    await expect(page.getByText('Connection & Runtime')).toBeVisible()
    await expect(page.getByText('Endpoint URL')).toBeVisible()
    await expect(
      page.getByRole('link', { name: /mcp\.example\.test\/e2e-detail\/sse/ }),
    ).toHaveAttribute('href', 'https://mcp.example.test/e2e-detail/sse')

    // Transport tile replaces Runtime for remote servers.
    await expect(page.getByText('Transport')).toBeVisible()
  })

  test('MCP detail page has Overview/Installation/Versions/JSON tabs', async ({ page }) => {
    await page.goto(`/mcp/${PUBLISHER_SLUG}/${MCP_SLUG}`)
    await expect(page.getByRole('heading', { name: MCP_NAME })).toBeVisible({ timeout: 15_000 })

    await expect(page.getByRole('tab', { name: /overview/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /installation/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /versions/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /json/i })).toBeVisible()

    // Installation tab shows the package identifier in the config generator.
    await page.getByRole('tab', { name: /installation/i }).click()
    await expect(page.getByText(/@e2e\/detail-mcp@1\.0\.0/).first()).toBeVisible({ timeout: 10_000 })

    // JSON tab shows the raw server document somewhere in its body.
    await page.getByRole('tab', { name: /json/i }).click()
    await expect(page.getByText(new RegExp(MCP_SLUG))).toBeVisible({ timeout: 10_000 })
  })

  // ── Agent detail page ─────────────────────────────────────────────────

  test('Agent detail page renders the name, Connection card, and A2A card link', async ({ page }) => {
    await page.goto(`/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByRole('heading', { name: AGENT_NAME })).toBeVisible({ timeout: 15_000 })

    // Connection card with the endpoint URL hero row.
    await expect(page.getByText('Connection')).toBeVisible()
    await expect(page.getByText('Endpoint URL')).toBeVisible()
    await expect(
      page.getByRole('link', { name: /agents\.example\.test\/e2e-detail/ }),
    ).toHaveAttribute('href', AGENT_ENDPOINT)

    // A2A protocol tile and authentication scheme.
    await expect(page.getByText('A2A Protocol')).toBeVisible()
    await expect(page.getByText('0.3.0')).toBeVisible()
    await expect(page.getByText('Authentication')).toBeVisible()
    await expect(page.getByText('Bearer').first()).toBeVisible()

    // A2A Agent Card link points at the well-known path for this agent.
    await expect(
      page.getByRole('link', { name: /a2a agent card/i }),
    ).toHaveAttribute(
      'href',
      `/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}/.well-known/agent-card.json`,
    )
  })

  test('Agent detail page exposes Skills/Connect/Versions/JSON tabs', async ({ page }) => {
    await page.goto(`/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}`)
    await expect(page.getByRole('heading', { name: AGENT_NAME })).toBeVisible({ timeout: 15_000 })

    await expect(page.getByRole('tab', { name: /overview/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /skills \(1\)/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /connect/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /versions/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /json/i })).toBeVisible()

    // Skills tab lists the seeded skill.
    await page.getByRole('tab', { name: /skills/i }).click()
    await expect(page.getByText('E2E Detail Skill')).toBeVisible({ timeout: 10_000 })
  })

  test('Agent well-known card endpoint serves a valid A2A card', async ({ page }) => {
    // Per openapi.yaml, the well-known card is served at
    // /agents/{ns}/{slug}/.well-known/agent-card.json (no /api/v1 prefix).
    const res = await page.request.get(
      `/agents/${PUBLISHER_SLUG}/${AGENT_SLUG}/.well-known/agent-card.json`,
    )
    expect(res.ok()).toBeTruthy()
    const body = (await res.json()) as { name?: string }
    expect(body.name).toBe(AGENT_NAME)
  })
})
