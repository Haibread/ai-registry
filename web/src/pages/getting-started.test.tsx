import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import GettingStartedPage from './getting-started'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ isAuthenticated: false, isLoading: false }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light' }),
}))

function renderPage() {
  return render(
    <MemoryRouter>
      <GettingStartedPage />
    </MemoryRouter>,
  )
}

describe('GettingStartedPage', () => {
  it('renders the heading', () => {
    renderPage()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/getting started/i)
  })

  it('renders breadcrumbs', () => {
    renderPage()
    // Breadcrumb "Home" is a link; trailing breadcrumb segment is rendered
    // inside the nav but is not itself a link. Scope assertions to the
    // breadcrumb nav so we don't collide with the header nav's own
    // "Getting Started" link.
    const crumbs = screen.getByRole('navigation', { name: /breadcrumb/i })
    expect(crumbs).toHaveTextContent('Home')
    expect(crumbs).toHaveTextContent('Getting Started')
  })

  it('renders the find a server section', () => {
    renderPage()
    expect(screen.getByText(/find a server or agent/i)).toBeInTheDocument()
  })

  it('renders the install section', () => {
    renderPage()
    expect(screen.getByText(/install or connect/i)).toBeInTheDocument()
  })

  it('renders the host config table', () => {
    renderPage()
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByText(/Claude Desktop/)).toBeInTheDocument()
  })
})
