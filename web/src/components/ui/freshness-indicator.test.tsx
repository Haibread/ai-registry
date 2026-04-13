import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { FreshnessIndicator } from './freshness-indicator'

describe('FreshnessIndicator', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-12T12:00:00Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows "Today" for same-day dates', () => {
    render(<FreshnessIndicator updatedAt="2026-04-12T10:00:00Z" />)
    expect(screen.getByText('Today')).toBeInTheDocument()
  })

  it('shows days for recent dates', () => {
    render(<FreshnessIndicator updatedAt="2026-04-09T12:00:00Z" />)
    expect(screen.getByText('3 days ago')).toBeInTheDocument()
  })

  it('shows weeks for dates 1-4 weeks old', () => {
    render(<FreshnessIndicator updatedAt="2026-03-29T12:00:00Z" />)
    expect(screen.getByText('2 weeks ago')).toBeInTheDocument()
  })

  it('shows green dot for dates < 3 months old', () => {
    const { container } = render(<FreshnessIndicator updatedAt="2026-03-01T12:00:00Z" />)
    const dot = container.querySelector('.bg-green-500')
    expect(dot).toBeInTheDocument()
  })

  it('shows yellow dot for dates 3-12 months old', () => {
    const { container } = render(<FreshnessIndicator updatedAt="2025-10-01T12:00:00Z" />)
    const dot = container.querySelector('.bg-yellow-500')
    expect(dot).toBeInTheDocument()
  })

  it('shows red dot and stale label for dates > 12 months old', () => {
    const { container } = render(<FreshnessIndicator updatedAt="2024-01-01T12:00:00Z" />)
    const dot = container.querySelector('.bg-red-500')
    expect(dot).toBeInTheDocument()
    expect(screen.getByText('(stale)')).toBeInTheDocument()
  })

  it('shows "1 year ago" for exactly 1 year', () => {
    render(<FreshnessIndicator updatedAt="2025-04-10T12:00:00Z" />)
    expect(screen.getByText('1 year ago')).toBeInTheDocument()
  })
})
