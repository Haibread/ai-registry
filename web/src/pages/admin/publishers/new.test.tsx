import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

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

import AdminPublisherNew from './new'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminPublisherNew />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminPublisherNew', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({ data: { id: 'pub-1' }, error: undefined })
  })

  it('renders the heading and core form fields', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /new publisher/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/slug/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/contact email/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /create publisher/i })).toBeInTheDocument()
  })

  it('submits POST with expected body when valid', async () => {
    const { container } = renderPage()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'my-org' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'My Org' } })
    fireEvent.change(container.querySelector('#contact') as HTMLInputElement, {
      target: { value: 'team@example.com' },
    })

    const form = container.querySelector('form')!
    fireEvent.submit(form)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/publishers', {
        body: { slug: 'my-org', name: 'My Org', contact: 'team@example.com' },
      })
    })
  })

  it('omits contact when blank', async () => {
    const { container } = renderPage()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'acme' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Acme' } })

    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/publishers', {
        body: { slug: 'acme', name: 'Acme', contact: undefined },
      })
    })
  })

  it('navigates on success', async () => {
    const { container } = renderPage()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'acme' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Acme' } })
    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/admin/publishers')
    })
  })

  it('shows error alert when POST returns an error', async () => {
    mockPOST.mockResolvedValueOnce({ data: undefined, error: { title: 'Slug already exists' } })
    const { container } = renderPage()
    fireEvent.change(container.querySelector('#slug') as HTMLInputElement, { target: { value: 'dup' } })
    fireEvent.change(container.querySelector('#name') as HTMLInputElement, { target: { value: 'Dup' } })
    fireEvent.submit(container.querySelector('form')!)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/slug already exists/i)
  })
})
