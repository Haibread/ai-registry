import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ComponentProps } from 'react'

const authFetch = vi.fn()
// Node's Request rejects the relative URLs the app's client uses in a browser.
vi.mock('@/lib/api-client', async () => {
  const { default: createClient } = await import('openapi-fetch')
  const client = createClient({ baseUrl: 'http://registry.test', fetch: (req: Request) => authFetch(req) })
  return { useAuthClient: () => client }
})

import { ProtocolVersionsInput } from './protocol-versions-input'

beforeEach(() => authFetch.mockReset())

function json(status: number, body: unknown, type = 'application/json') {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } })
}

const remote = { namespace: 'acme', transport: 'streamable_http', remoteUrl: 'https://mcp.acme.dev/github' }

function renderInput(props: ComponentProps<typeof ProtocolVersionsInput> = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const utils = render(
    <QueryClientProvider client={qc}>
      <ProtocolVersionsInput {...props} />
    </QueryClientProvider>,
  )
  const value = () =>
    JSON.parse((utils.container.querySelector('input[name="protocol_versions"]') as HTMLInputElement).value)
  return { ...utils, value, field: screen.getByLabelText('Protocol versions') }
}

describe('ProtocolVersionsInput', () => {
  it('submits the default revisions newest first', () => {
    const { value } = renderInput({ defaultValue: ['2025-03-26', '2025-06-18'] })
    expect(value()).toEqual(['2025-06-18', '2025-03-26'])
  })

  it('adds a revision on Enter and keeps the list sorted', async () => {
    const { value, field } = renderInput({ defaultValue: ['2025-03-26'] })
    await userEvent.type(field, '2025-11-25{Enter}')
    expect(value()).toEqual(['2025-11-25', '2025-03-26'])
    expect(field).toHaveValue('')
  })

  it.each([
    ['latest', /YYYY-MM-DD/],
    ['2025-13-40', /YYYY-MM-DD/],
    ['2025-03-26', /already in the list/],
  ])('rejects %s', async (typed, message) => {
    const { value, field } = renderInput({ defaultValue: ['2025-03-26'] })
    await userEvent.type(field, `${typed}{Enter}`)
    expect(screen.getByRole('alert')).toHaveTextContent(message)
    expect(value()).toEqual(['2025-03-26'])
  })

  it('commits a typed revision when the field loses focus', async () => {
    const { value, field } = renderInput({ defaultValue: ['2025-03-26'] })
    await userEvent.type(field, '2025-06-18')
    await userEvent.tab()
    expect(value()).toEqual(['2025-06-18', '2025-03-26'])
  })

  it('removes a revision but never the last one', async () => {
    const { value } = renderInput({ defaultValue: ['2025-06-18', '2025-03-26'] })
    await userEvent.click(screen.getByRole('button', { name: 'Remove 2025-06-18' }))
    expect(value()).toEqual(['2025-03-26'])
    await userEvent.click(screen.getByRole('button', { name: 'Remove 2025-03-26' }))
    expect(value()).toEqual(['2025-03-26'])
    expect(screen.getByRole('alert')).toHaveTextContent(/at least one/)
  })

  it('has no detection without a discovery source', () => {
    renderInput()
    expect(screen.queryByRole('button', { name: /detect from server/i })).not.toBeInTheDocument()
  })

  it.each([
    [{ ...remote, transport: 'stdio' }],
    [{ ...remote, remoteUrl: '' }],
  ])('disables detection without a remote endpoint (%o)', (discovery) => {
    renderInput({ discovery })
    expect(screen.getByRole('button', { name: /detect from server/i })).toBeDisabled()
  })

  it('offers the detected revisions and replaces the list on confirm', async () => {
    authFetch.mockResolvedValue(
      json(200, {
        endpoint: { url: 'https://mcp.acme.dev/github/mcp', transport: 'streamable_http' },
        attempts: [],
        protocol_version: '2025-11-25',
        supported_protocol_versions: ['2025-11-25', '2025-06-18', '2025-03-26'],
        tools: [],
      }),
    )
    const { value } = renderInput({ defaultValue: ['2024-11-05'], discovery: remote })

    await userEvent.click(screen.getByRole('button', { name: /detect from server/i }))
    expect(await screen.findByText(/accepts 3 revisions/)).toBeInTheDocument()
    const req = authFetch.mock.calls[0][0] as Request
    expect(await req.json()).toEqual({ namespace: 'acme', url: remote.remoteUrl, transport: 'streamable_http' })
    expect(value()).toEqual(['2024-11-05'])

    await userEvent.click(screen.getByRole('button', { name: 'Use these 3' }))
    expect(value()).toEqual(['2025-11-25', '2025-06-18', '2025-03-26'])
  })

  it('keeps the list and suggests the detected revisions instead', async () => {
    authFetch.mockResolvedValue(
      json(200, {
        endpoint: { url: remote.remoteUrl, transport: 'streamable_http' },
        attempts: [],
        protocol_version: '2025-06-18',
        supported_protocol_versions: ['2025-06-18', '2025-03-26'],
        tools: [],
      }),
    )
    const { value, field } = renderInput({ defaultValue: ['2025-03-26'], discovery: remote })

    await userEvent.click(screen.getByRole('button', { name: /detect from server/i }))
    await userEvent.click(await screen.findByRole('button', { name: 'Keep my list' }))
    expect(value()).toEqual(['2025-03-26'])

    await userEvent.type(field, '2025')
    await userEvent.click(screen.getByRole('button', { name: /^2025-06-18\s*supported by the server$/ }))
    expect(value()).toEqual(['2025-06-18', '2025-03-26'])
  })

  it('explains a server that requires authentication', async () => {
    authFetch.mockResolvedValue(
      json(
        502,
        { type: 'https://registry.test/problems/upstream-unauthorized', title: 'Bad Gateway', status: 502 },
        'application/problem+json',
      ),
    )
    renderInput({ discovery: remote })
    await userEvent.click(screen.getByRole('button', { name: /detect from server/i }))
    expect(await screen.findByText('This server requires authentication.')).toBeInTheDocument()
  })
})
