/**
 * new-version-form.test.tsx
 *
 * The contract under test is the prefill behavior: when the parent passes the
 * latest existing version, the form seeds every field from it (so authoring
 * v(n+1) is a small delta), suggests a patch-bumped version number, and says
 * where the values came from. Submit wiring is covered by the e2e journeys.
 */

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ComponentProps } from 'react'

vi.mock('@/auth/tokens', () => ({
  authFetch: vi.fn(),
}))

import { NewVersionForm } from './new-version-form'

function renderForm(props: Partial<ComponentProps<typeof NewVersionForm>> = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <NewVersionForm
          kind="mcp"
          namespace="acme"
          slug="weather"
          onCreated={() => {}}
          onCancel={() => {}}
          {...props}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const mcpPrefill = {
  id: '01HV1',
  version: '1.2.3',
  runtime: 'sse' as const,
  protocol_version: '2025-03-26',
  packages: [
    {
      registryType: 'npm',
      registryBaseUrl: 'https://registry.npmjs.org',
      identifier: '@acme/weather',
      version: '1.2.3',
      transport: { type: 'sse' as const, url: 'https://pkg.example.com/sse' },
    },
  ],
  remotes: [{ type: 'sse' as const, url: 'https://mcp.example.com/sse' }],
  capabilities: { tools: { listChanged: true } },
  tools: [{ name: 'get_forecast' }, { name: 'get_alerts' }],
  status: 'active' as const,
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
}

const agentPrefill = {
  id: '01HV2',
  version: '2.0.0',
  endpoint_url: 'https://agent.example.com',
  protocol_version: '0.2.1',
  skills: [
    {
      id: 'summarize',
      name: 'Summarize text',
      description: 'TL;DR anything',
      tags: ['text', 'nlp'],
      examples: ['Summarize this article', 'Give me a TL;DR'],
    },
  ],
  // The OpenAPI spec types authentication items as free-form objects (no
  // declared keys), so the generated type is Record<string, never> — go
  // through unknown to hand it a realistic {scheme} payload.
  authentication: [{ scheme: 'Bearer' }] as unknown as Record<string, never>[],
  default_input_modes: ['text/plain', 'application/json'],
  default_output_modes: ['application/json'],
  provider: { organization: 'Acme Inc.', url: 'https://acme.example.com' },
  documentation_url: 'https://docs.example.com',
  icon_url: 'https://acme.example.com/icon.png',
  status: 'active' as const,
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
}

describe('NewVersionForm prefill (mcp)', () => {
  it('starts blank without a prefill', () => {
    renderForm()
    expect(screen.getByLabelText(/^version/i)).toHaveValue('')
    expect(screen.queryByText(/pre-filled from/i)).not.toBeInTheDocument()
    // stdio default → no remote endpoint field.
    expect(screen.queryByLabelText(/remote endpoint url/i)).not.toBeInTheDocument()
  })

  it('seeds every field from the previous version and bumps the patch', () => {
    renderForm({ prefill: mcpPrefill })

    expect(screen.getByText(/pre-filled from/i)).toHaveTextContent('v1.2.3')
    expect(screen.getByLabelText(/^version/i)).toHaveValue('1.2.4')
    expect(screen.getByLabelText(/protocol version/i)).toHaveValue('2025-03-26')

    // Transport carried over → SSE, which reveals the seeded remote endpoint.
    expect(screen.getByRole('combobox', { name: /transport/i })).toHaveTextContent('SSE')
    expect(screen.getByLabelText(/remote endpoint url/i)).toHaveValue('https://mcp.example.com/sse')

    expect(screen.getByLabelText(/package identifier/i)).toHaveValue('@acme/weather')
    expect(screen.getByLabelText(/package version/i)).toHaveValue('1.2.3')
    expect(screen.getByLabelText(/package url/i)).toHaveValue('https://pkg.example.com/sse')
    expect(screen.getByLabelText(/registry base url/i)).toHaveValue('https://registry.npmjs.org')

    expect(screen.getByLabelText(/capabilities/i)).toHaveValue(
      JSON.stringify({ tools: { listChanged: true } }, null, 2),
    )

    // Tools land in the editor (count chip + hidden bridge input).
    expect(screen.getByText(/2 tools/i)).toBeInTheDocument()
    const hidden = document.querySelector('input[name="tools"]') as HTMLInputElement
    expect(JSON.parse(hidden.value)).toEqual([{ name: 'get_forecast' }, { name: 'get_alerts' }])
  })

  it('leaves the version blank when the previous one is not semver', () => {
    renderForm({ prefill: { ...mcpPrefill, version: 'not-semver' } })
    expect(screen.getByLabelText(/^version/i)).toHaveValue('')
  })
})

describe('NewVersionForm prefill (agent)', () => {
  it('seeds endpoint, skill, auth, modes, and provider metadata', () => {
    renderForm({ kind: 'agent', prefill: agentPrefill })

    expect(screen.getByLabelText(/^version/i)).toHaveValue('2.0.1')
    expect(screen.getByLabelText(/endpoint url/i)).toHaveValue('https://agent.example.com')

    expect(screen.getByLabelText(/skill id/i)).toHaveValue('summarize')
    expect(screen.getByLabelText(/skill name/i)).toHaveValue('Summarize text')
    expect(screen.getByLabelText(/skill description/i)).toHaveValue('TL;DR anything')
    expect(screen.getByLabelText(/skill tags/i)).toHaveValue('text, nlp')
    expect(screen.getByLabelText(/skill examples/i)).toHaveValue(
      'Summarize this article\nGive me a TL;DR',
    )

    expect(screen.getByRole('combobox', { name: /authentication/i })).toHaveTextContent('Bearer')

    // Mode checkboxes mirror the previous version's arrays, not the defaults.
    const inputModes = screen.getByRole('group', { name: /input modes/i })
    expect(inputModes).toBeInTheDocument()
    const json = document.querySelector('input[name="input_mode_application/json"]') as HTMLInputElement
    expect(json.checked).toBe(true)
    const outPlain = document.querySelector('input[name="output_mode_text/plain"]') as HTMLInputElement
    expect(outPlain.checked).toBe(false)

    expect(screen.getByLabelText(/provider organization/i)).toHaveValue('Acme Inc.')
    expect(screen.getByLabelText(/provider url/i)).toHaveValue('https://acme.example.com')
    expect(screen.getByLabelText(/documentation url/i)).toHaveValue('https://docs.example.com')
    expect(screen.getByLabelText(/icon url/i)).toHaveValue('https://acme.example.com/icon.png')
  })
})
