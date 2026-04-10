import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { SessionExpired } from './SessionExpired'

const mockSignoutRedirect = vi.fn()

vi.mock('./AuthContext', () => ({
  getUserManager: () => Promise.resolve({ signoutRedirect: mockSignoutRedirect }),
}))

beforeEach(() => vi.clearAllMocks())

describe('<SessionExpired>', () => {
  it('renders session expired message', () => {
    render(<SessionExpired />)
    expect(screen.getByText(/session expired/i)).toBeInTheDocument()
  })

  it('mentions signing you out', () => {
    render(<SessionExpired />)
    expect(screen.getByText(/signing you out/i)).toBeInTheDocument()
  })

  it('calls signoutRedirect on mount', async () => {
    await act(async () => {
      render(<SessionExpired />)
    })
    expect(mockSignoutRedirect).toHaveBeenCalledTimes(1)
  })

  it('does not call signoutRedirect more than once on re-render', async () => {
    let rerender: (ui: React.ReactElement) => void
    await act(async () => {
      ;({ rerender } = render(<SessionExpired />))
    })
    await act(async () => {
      rerender(<SessionExpired />)
    })
    expect(mockSignoutRedirect).toHaveBeenCalledTimes(1)
  })
})
