import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SectionHeader } from './section-header'

describe('SectionHeader', () => {
  it('renders the title', () => {
    render(<SectionHeader title="Runtime" />)
    expect(screen.getByRole('heading', { name: 'Runtime' })).toBeInTheDocument()
  })

  it('does not render icon wrapper when no icon is provided', () => {
    const { container } = render(<SectionHeader title="Release" />)
    // Only the <h2> should be inside the container's first child
    const root = container.firstChild as HTMLElement
    expect(root.querySelectorAll('span').length).toBe(0)
  })

  it('renders a provided icon', () => {
    render(
      <SectionHeader
        title="Connection"
        icon={<svg data-testid="icon" />}
      />,
    )
    expect(screen.getByTestId('icon')).toBeInTheDocument()
  })

  it('applies uppercase styling classes to the heading', () => {
    render(<SectionHeader title="Activity" />)
    const heading = screen.getByRole('heading', { name: 'Activity' })
    expect(heading.className).toContain('uppercase')
  })
})
