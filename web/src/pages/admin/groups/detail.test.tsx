import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPATCH = vi.fn()
const mockPUT = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, PATCH: mockPATCH, PUT: mockPUT, DELETE: mockDELETE }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import AdminGroupDetail from './detail'

const group = { id: '01HG1', slug: 'platform', name: 'Platform', description: 'infra', created_at: '2026-04-01T10:00:00Z', updated_at: '2026-04-01T10:00:00Z' }
const members = [{ id: '01HU1', email: 'a@x.test', display_name: 'Ada', has_password: false, is_server_admin: false, disabled: false, created_at: '2026-04-01T10:00:00Z', updated_at: '2026-04-01T10:00:00Z' }]

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/groups/platform']}>
        <Routes>
          <Route path="/admin/groups/:slug" element={<AdminGroupDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminGroupDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/groups/{slug}') return Promise.resolve({ data: group })
      if (path === '/api/v1/groups/{slug}/members') return Promise.resolve({ data: { items: members } })
      return Promise.resolve({ data: undefined })
    })
  })

  it('renders the group and its members', async () => {
    renderPage()
    expect(await screen.findByText('Platform')).toBeInTheDocument()
    expect(await screen.findByText('a@x.test')).toBeInTheDocument()
  })

  it('adds a member by email via PUT', async () => {
    mockPUT.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Platform')
    await userEvent.type(screen.getByLabelText(/add member by email/i), 'new@x.test')
    await userEvent.click(screen.getByRole('button', { name: /^add$/i }))
    await waitFor(() => {
      expect(mockPUT).toHaveBeenCalledWith('/api/v1/groups/{slug}/members/{email}', {
        params: { path: { slug: 'platform', email: 'new@x.test' } },
      })
    })
  })

  it('removes a member via DELETE', async () => {
    mockDELETE.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('a@x.test')
    await userEvent.click(screen.getByRole('button', { name: /remove a@x.test/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith('/api/v1/groups/{slug}/members/{email}', {
        params: { path: { slug: 'platform', email: 'a@x.test' } },
      })
    })
  })
})
