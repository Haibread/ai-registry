import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

const setTheme = vi.fn()
let resolved: 'light' | 'dark' = 'light'

vi.mock('@/components/providers', () => ({
  useTheme: () => ({ resolvedTheme: resolved, setTheme, theme: resolved }),
}))

import { ThemeToggle } from './theme-toggle'

describe('ThemeToggle', () => {
  beforeEach(() => {
    setTheme.mockReset()
    resolved = 'light'
  })

  it('renders moon icon and dark-mode aria label when light', () => {
    resolved = 'light'
    render(<ThemeToggle />)
    const btn = screen.getByRole('button', { name: /switch to dark mode/i })
    expect(btn).toBeInTheDocument()
  })

  it('renders sun icon and light-mode aria label when dark', () => {
    resolved = 'dark'
    render(<ThemeToggle />)
    expect(screen.getByRole('button', { name: /switch to light mode/i })).toBeInTheDocument()
  })

  it('calls setTheme("dark") when clicked in light mode', () => {
    resolved = 'light'
    render(<ThemeToggle />)
    fireEvent.click(screen.getByRole('button'))
    expect(setTheme).toHaveBeenCalledWith('dark')
  })

  it('calls setTheme("light") when clicked in dark mode', () => {
    resolved = 'dark'
    render(<ThemeToggle />)
    fireEvent.click(screen.getByRole('button'))
    expect(setTheme).toHaveBeenCalledWith('light')
  })
})
