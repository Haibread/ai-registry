import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Radix Select pointer-API shims live in src/test/setup.ts — they are
// applied once at vitest startup, no per-file boilerplate needed.

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token', clearSession: vi.fn() }),
}))

// The publisher dropdown is scoped to what the caller can author on: the list
// comes from PublisherContext (derived from grants) and is filtered by
// usePermissions().canEdit. Both are mocked so the page renders standalone.
let mockPublishers: { slug: string; name: string; roles: string[] }[]
let mockCanEdit: (slug: string | undefined) => boolean
vi.mock('@/auth/PublisherContext', () => ({
  usePublisher: () => ({ publishers: mockPublishers }),
}))
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => ({ canEdit: mockCanEdit }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST }),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

import AdminMCPNew from './new'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminMCPNew />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

async function selectNamespace() {
  // shadcn Select in jsdom: click trigger, then click the option by role
  const trigger = screen.getByLabelText(/namespace/i)
  fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' })
  fireEvent.click(trigger)
  const option = await screen.findByRole('option', { name: /acme/i })
  fireEvent.click(option)
}

describe('AdminMCPNew', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPublishers = [{ slug: 'acme', name: 'Acme', roles: ['editor'] }]
    mockCanEdit = () => true
    mockPOST.mockResolvedValue({ data: { id: 'srv-1' }, error: undefined })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({}),
    }) as unknown as typeof fetch
  })

  it('renders the heading and core form fields', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /new mcp server/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/slug/i)).toBeInTheDocument()
    expect(document.getElementById('name')).toBeInTheDocument()
    expect(screen.getByLabelText(/description/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/repository url/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/publish version immediately/i)).toBeInTheDocument()
  })

  it('disables submit while no namespace selected', () => {
    renderPage()
    const submit = screen.getByRole('button', { name: /create mcp server/i })
    expect(submit).toBeDisabled()
  })

  it('submits POST with expected body when valid', async () => {
    const { container } = renderPage()

    await selectNamespace()

    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    // Clear version so no fetch call is needed
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '' } })
    // Uncheck publish so publish endpoint isn't called
    fireEvent.click(container.querySelector('#publish') as HTMLInputElement)

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/mcp/servers', {
        body: {
          namespace: 'acme',
          slug: 'my-srv',
          name: 'My Server',
          description: undefined,
          homepage_url: undefined,
          repo_url: undefined,
          license: undefined,
        },
      })
    })
  })

  it('calls publish endpoint via fetch when publish checkbox is checked', async () => {
    const { container } = renderPage()

    await selectNamespace()

    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
      const urls = fetchMock.mock.calls.map((c) => c[0])
      expect(urls).toContain('/api/v1/mcp/servers/acme/my-srv/versions')
      expect(urls).toContain('/api/v1/mcp/servers/acme/my-srv/versions/1.0.0/publish')
    })
  })

  it('sends the parsed tools array in the version POST body', async () => {
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })
    fireEvent.change(container.querySelector('#tools') as HTMLTextAreaElement, {
      target: {
        value: JSON.stringify([
          { name: 'read_file', description: 'Read a file' },
          { name: 'write_file', description: 'Write a file' },
        ]),
      },
    })

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
      const versionCall = fetchMock.mock.calls.find(
        (c) => c[0] === '/api/v1/mcp/servers/acme/my-srv/versions',
      )
      expect(versionCall).toBeDefined()
      const body = JSON.parse((versionCall![1] as RequestInit).body as string)
      expect(body.tools).toEqual([
        { name: 'read_file', description: 'Read a file' },
        { name: 'write_file', description: 'Write a file' },
      ])
    })
  })

  it('omits the tools field when the textarea is empty', async () => {
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })
    // tools field intentionally left blank

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
      const versionCall = fetchMock.mock.calls.find(
        (c) => c[0] === '/api/v1/mcp/servers/acme/my-srv/versions',
      )
      expect(versionCall).toBeDefined()
      const body = JSON.parse((versionCall![1] as RequestInit).body as string)
      expect(body).not.toHaveProperty('tools')
    })
  })

  it('shows a parse error when tools is invalid JSON', async () => {
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })
    fireEvent.change(container.querySelector('#tools') as HTMLTextAreaElement, {
      target: { value: 'not-valid-json{' },
    })

    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/tools field must be valid json/i)
  })

  it('rejects a non-array tools value client-side', async () => {
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-srv' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Server' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })
    fireEvent.change(container.querySelector('#tools') as HTMLTextAreaElement, {
      target: { value: '{"name":"oops"}' },
    })

    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/tools field must be a json array/i)
  })

  it('shows error alert when POST returns an error', async () => {
    mockPOST.mockResolvedValueOnce({ data: undefined, error: { title: 'Slug already exists' } })
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'dup' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Dup' } })

    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/slug already exists/i)
  })

  it('lists only publishers the caller can author on', async () => {
    // Caller holds a grant on both publishers but can only author on acme
    // (viewer-only on globex). The dropdown must hide globex.
    mockPublishers = [
      { slug: 'acme', name: 'Acme', roles: ['editor'] },
      { slug: 'globex', name: 'Globex', roles: ['viewer'] },
    ]
    mockCanEdit = (slug) => slug === 'acme'
    renderPage()

    const trigger = screen.getByLabelText(/namespace/i)
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' })
    fireEvent.click(trigger)

    expect(await screen.findByRole('option', { name: /acme/i })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /globex/i })).not.toBeInTheDocument()
  })
})
