import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ActivityStrip } from './activity-strip'

describe('ActivityStrip', () => {
  const base = {
    viewCount: 5,
    copyCount: 2,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-06-01T00:00:00Z',
  }

  it('renders section header', () => {
    render(<ActivityStrip {...base} />)
    expect(screen.getByText('Activity')).toBeInTheDocument()
  })

  it('renders view and install counts with proper pluralization', () => {
    render(<ActivityStrip {...base} />)
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.getByText(/views/)).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText(/installs/)).toBeInTheDocument()
  })

  it('uses singular forms when count is 1', () => {
    render(<ActivityStrip {...base} viewCount={1} copyCount={1} />)
    expect(screen.getByText(/^view$/)).toBeInTheDocument()
    expect(screen.getByText(/^install$/)).toBeInTheDocument()
  })

  it('formats large numbers with locale separators', () => {
    render(<ActivityStrip {...base} viewCount={1234567} copyCount={0} />)
    // Either "1,234,567" or locale equivalent
    const formatted = (1234567).toLocaleString()
    expect(screen.getByText(formatted)).toBeInTheDocument()
  })

  it('renders Created and Updated labels', () => {
    render(<ActivityStrip {...base} />)
    expect(screen.getByText(/Created/)).toBeInTheDocument()
    expect(screen.getByText(/Updated/)).toBeInTheDocument()
  })
})
