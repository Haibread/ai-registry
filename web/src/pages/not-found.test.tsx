import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

import NotFoundPage from './not-found'

describe('NotFoundPage', () => {
  it('shows a 404 with a way back home', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/nowhere']}>
          <NotFoundPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(screen.getByRole('heading', { name: '404' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Go home' })).toHaveAttribute('href', '/')
  })
})
