import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => mockNavigate,
}))

const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ POST: mockPOST }),
}))

import AdminGroupNew from './new'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminGroupNew />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminGroupNew', () => {
  beforeEach(() => vi.clearAllMocks())

  it('submits slug + name and navigates on success', async () => {
    mockPOST.mockResolvedValue({ data: { id: '01HG1' }, error: undefined })
    renderPage()

    await userEvent.type(screen.getByLabelText(/slug/i), 'platform')
    await userEvent.type(screen.getByLabelText(/^name/i), 'Platform Team')
    await userEvent.click(screen.getByRole('button', { name: /create group/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/groups', {
        body: { slug: 'platform', name: 'Platform Team', description: undefined },
      })
    })
    expect(mockNavigate).toHaveBeenCalledWith('/admin/groups')
  })

  it('shows an error when the API returns a conflict', async () => {
    mockPOST.mockResolvedValue({ data: undefined, error: { title: 'a group with that slug already exists' } })
    renderPage()
    await userEvent.type(screen.getByLabelText(/slug/i), 'dup')
    await userEvent.type(screen.getByLabelText(/^name/i), 'Dup')
    await userEvent.click(screen.getByRole('button', { name: /create group/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/already in use|already exists/i)
    expect(mockNavigate).not.toHaveBeenCalled()
  })
})
