/**
 * new-version-form.test.tsx
 *
 * The contract under test is the prefill behavior: when the parent passes the
 * latest existing version, the form seeds every field from it (so authoring
 * v(n+1) is a small delta), suggests a patch-bumped version number, and says
 * where the values came from. The submit test covers the payload shaping the form owns; the round-trip is
 * covered by the e2e journeys.
 */

import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ComponentProps } from 'react'

vi.mock('@/auth/tokens', () => ({
  authFetch: vi.fn(),
}))

import { authFetch } from '@/auth/tokens'
import { NewVersionForm } from './new-version-form'

function renderForm(props: Partial<ComponentProps<typeof NewVersionForm>> = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <NewVersionForm
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
  protocol_versions: ['2025-06-18', '2025-03-26'],
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

describe('NewVersionForm prefill', () => {
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
    expect(screen.getByRole('button', { name: 'Remove 2025-06-18' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Remove 2025-03-26' })).toBeInTheDocument()

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

describe('NewVersionForm submit', () => {
  it('sends the protocol revision tokens as a list', async () => {
    vi.mocked(authFetch).mockResolvedValue(new Response('{}', { status: 201 }))
    renderForm()

    fireEvent.change(screen.getByLabelText(/^version/i), { target: { value: '1.0.0' } })
    const revisions = screen.getByLabelText(/protocol versions/i)
    fireEvent.change(revisions, { target: { value: '2025-06-18' } })
    fireEvent.keyDown(revisions, { key: 'Enter' })
    fireEvent.submit(screen.getByLabelText(/^version/i).closest('form')!)

    await waitFor(() => expect(authFetch).toHaveBeenCalled())
    const [, init] = vi.mocked(authFetch).mock.calls[0]
    expect(JSON.parse(init!.body as string).protocol_versions).toEqual(['2025-06-18', '2025-03-26'])
  })
})
