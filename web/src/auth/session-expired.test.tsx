import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SessionExpired } from './SessionExpired'

vi.mock('./AuthContext', () => ({
  userManager: { signoutRedirect: vi.fn() },
}))

import { userManager } from './AuthContext'
const mockSignout = vi.mocked(userManager.signoutRedirect)

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

  it('calls signoutRedirect on mount', () => {
    render(<SessionExpired />)
    expect(mockSignout).toHaveBeenCalledTimes(1)
  })

  it('does not call signoutRedirect more than once on re-render', () => {
    const { rerender } = render(<SessionExpired />)
    rerender(<SessionExpired />)
    expect(mockSignout).toHaveBeenCalledTimes(1)
  })
})
