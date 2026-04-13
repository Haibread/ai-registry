import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatTile } from './stat-tile'

describe('StatTile', () => {
  it('renders its label and child value', () => {
    render(
      <StatTile label="Runtime">
        <span>stdio</span>
      </StatTile>,
    )
    expect(screen.getByText('Runtime')).toBeInTheDocument()
    expect(screen.getByText('stdio')).toBeInTheDocument()
  })

  it('renders the optional icon when provided', () => {
    render(
      <StatTile label="Protocol" icon={<svg data-testid="tile-icon" />}>
        1.0.0
      </StatTile>,
    )
    expect(screen.getByTestId('tile-icon')).toBeInTheDocument()
  })

  it('omits the icon slot when no icon is provided', () => {
    const { container } = render(
      <StatTile label="Protocol">1.0.0</StatTile>,
    )
    // The icon wrapper span is only emitted when `icon` is truthy, so none of
    // the spans inside the tile should carry the icon class prefix.
    expect(container.querySelectorAll('span.flex.items-center.text-muted-foreground\\/60').length).toBe(0)
  })

  it('renders a tooltip trigger when a tooltip is provided', () => {
    render(
      <StatTile label="Transport" tooltip="How the client reaches the server">
        sse
      </StatTile>,
    )
    // TooltipInfo renders a button with aria-label="More information".
    expect(screen.getByRole('button', { name: /more information/i })).toBeInTheDocument()
  })

  it('does not render a tooltip trigger when no tooltip is provided', () => {
    render(
      <StatTile label="Transport">sse</StatTile>,
    )
    expect(screen.queryByRole('button', { name: /more information/i })).not.toBeInTheDocument()
  })

  it('merges a custom className onto the wrapper', () => {
    const { container } = render(
      <StatTile label="Status" className="col-span-2">
        Published
      </StatTile>,
    )
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper.className).toMatch(/\bcol-span-2\b/)
    // Baseline classes from the component should still be present.
    expect(wrapper.className).toMatch(/\bspace-y-1\.5\b/)
    expect(wrapper.className).toMatch(/\bmin-w-0\b/)
  })
})
