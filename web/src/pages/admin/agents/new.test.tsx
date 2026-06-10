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
let mockCanReview: (slug: string | undefined) => boolean
vi.mock('@/auth/PublisherContext', () => ({
  usePublisher: () => ({ publishers: mockPublishers }),
}))
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => ({ canEdit: mockCanEdit, canReview: mockCanReview }),
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

import AdminAgentNew from './new'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminAgentNew />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

async function selectNamespace() {
  const trigger = screen.getByLabelText(/publisher/i)
  fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' })
  fireEvent.click(trigger)
  const option = await screen.findByRole('option', { name: /acme/i })
  fireEvent.click(option)
}

describe('AdminAgentNew', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPublishers = [{ slug: 'acme', name: 'Acme', roles: ['editor'] }]
    mockCanEdit = () => true
    mockCanReview = () => true
    mockPOST.mockResolvedValue({ data: { id: 'agent-1' }, error: undefined })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({}),
    }) as unknown as typeof fetch
  })

  it('renders the heading and core form fields', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /new agent/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/slug/i)).toBeInTheDocument()
    expect(document.getElementById('name')).toBeInTheDocument()
    expect(screen.getByLabelText(/endpoint url/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/publish version immediately/i)).toBeInTheDocument()
  })

  it('disables submit while no namespace selected', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /create agent/i })).toBeDisabled()
  })

  it('submits POST with expected body for create-agent flow', async () => {
    const { container } = renderPage()

    await selectNamespace()

    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-agent' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Agent' } })
    fireEvent.change(container.querySelector('#description') as HTMLInputElement, {
      target: { value: 'Does things' },
    })
    // Leave version blank so version-creation fetch is skipped
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '' } })
    // Uncheck publish so publish fetch is not called
    fireEvent.click(container.querySelector('#publish') as HTMLInputElement)

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/agents', {
        body: {
          namespace: 'acme',
          slug: 'my-agent',
          name: 'My Agent',
          description: 'Does things',
        },
      })
    })
  })

  it('invalidates the admin list cache on successful create', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    const { container } = render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <AdminAgentNew />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-agent' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Agent' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '' } })
    fireEvent.click(container.querySelector('#publish') as HTMLInputElement)

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin-agents'] })
    })
  })

  it('creates a version via fetch when version + endpoint_url are supplied', async () => {
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'a1' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'A1' } })
    fireEvent.change(container.querySelector('#version') as HTMLInputElement, { target: { value: '1.0.0' } })
    fireEvent.change(container.querySelector('#endpoint_url') as HTMLInputElement, {
      target: { value: 'https://api.example.com/agent' },
    })

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
      const urls = fetchMock.mock.calls.map((c) => c[0])
      expect(urls).toContain('/api/v1/agents/acme/a1/versions')
    })
  })

  it('shows error alert when POST returns an error', async () => {
    mockPOST.mockResolvedValueOnce({ data: undefined, error: { title: 'Slug taken' } })
    const { container } = renderPage()

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'dup' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Dup' } })

    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/slug taken/i)
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

    const trigger = screen.getByLabelText(/publisher/i)
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' })
    fireEvent.click(trigger)

    expect(await screen.findByRole('option', { name: /acme/i })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /globex/i })).not.toBeInTheDocument()
  })
})
