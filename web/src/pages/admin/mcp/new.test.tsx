import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Radix Select pointer-API shims live in src/test/setup.ts — they are
// applied once at vitest startup, no per-file boilerplate needed.

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token', clearSession: vi.fn() }),
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
    mockGET.mockResolvedValue({
      data: { items: [{ id: 'pub-1', slug: 'acme', name: 'Acme' }] },
    })
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
    await waitFor(() => expect(mockGET).toHaveBeenCalled())

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
    await waitFor(() => expect(mockGET).toHaveBeenCalled())

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

  it('shows error alert when POST returns an error', async () => {
    mockPOST.mockResolvedValueOnce({ data: undefined, error: { title: 'Slug already exists' } })
    const { container } = renderPage()
    await waitFor(() => expect(mockGET).toHaveBeenCalled())

    await selectNamespace()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'dup' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Dup' } })

    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/slug already exists/i)
  })
})
