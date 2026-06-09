import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

interface MockPub {
  currentSlug: string | null
  current: { slug: string; name: string; roles: string[] } | null
}
let mockPub: MockPub
vi.mock('@/auth/PublisherContext', () => ({ usePublisher: () => mockPub }))

interface MockPerms {
  canAdmin: (slug: string) => boolean
}
let mockPerms: MockPerms
vi.mock('@/auth/useMe', () => ({ usePermissions: () => mockPerms }))

vi.mock('@/auth/AuthContext', () => ({ useAuth: () => ({ accessToken: 't' }) }))

const mockGET = vi.fn()
const mockPATCH = vi.fn()
vi.mock('@/lib/api-client', () => ({ useAuthClient: () => ({ GET: mockGET, PATCH: mockPATCH }) }))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import AdminSettings from './settings'

function renderPage() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminSettings />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockPub = { currentSlug: 'acme', current: { slug: 'acme', name: 'Acme', roles: ['admin'] } }
  mockPerms = { canAdmin: () => true }
  mockGET.mockResolvedValue({
    data: { slug: 'acme', name: 'Acme Corp', contact: 'ops@acme.example' },
  })
  mockPATCH.mockResolvedValue({ data: {}, error: null })
})

describe('AdminSettings', () => {
  it('prompts to select a publisher when none is chosen', () => {
    mockPub = { currentSlug: null, current: null }
    renderPage()
    expect(screen.getByText(/select a publisher/i)).toBeInTheDocument()
  })

  it('shows a read-only view to a non-admin member (no Save button, inputs disabled)', async () => {
    mockPerms = { canAdmin: () => false }
    renderPage()
    const name = await screen.findByLabelText(/name/i)
    expect(name).toBeDisabled()
    expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument()
    expect(screen.getByText(/need the admin role/i)).toBeInTheDocument()
  })

  it('prefills the form from the loaded publisher and keeps the slug read-only', async () => {
    renderPage()
    const name = await screen.findByLabelText(/name/i)
    expect(name).toHaveValue('Acme Corp')
    expect(screen.getByLabelText(/contact/i)).toHaveValue('ops@acme.example')
    const slug = screen.getByLabelText(/slug/i)
    expect(slug).toHaveValue('acme')
    expect(slug).toBeDisabled()
  })

  it('PATCHes the publisher with the edited name + contact on save', async () => {
    const user = userEvent.setup()
    renderPage()
    const name = await screen.findByLabelText(/name/i)
    await user.clear(name)
    await user.type(name, 'Acme Renamed')
    await user.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => expect(mockPATCH).toHaveBeenCalledTimes(1))
    expect(mockPATCH).toHaveBeenCalledWith('/api/v1/publishers/{slug}', {
      params: { path: { slug: 'acme' } },
      body: { name: 'Acme Renamed', contact: 'ops@acme.example' },
    })
  })
})
