import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

import AdminGrants from './grants'

describe('AdminGrants', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [] } })
  })

  it('renders the global grants heading and the grants section', () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <AdminGrants />
      </QueryClientProvider>,
    )
    expect(screen.getByRole('heading', { name: /global grants/i })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /role grants/i })).toBeInTheDocument()
  })
})
