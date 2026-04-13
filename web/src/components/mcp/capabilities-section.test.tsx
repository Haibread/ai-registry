import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { CapabilitiesSection } from './capabilities-section'

describe('CapabilitiesSection', () => {
  it('renders nothing when capabilities is empty', () => {
    const { container } = render(<CapabilitiesSection capabilities={{}} />)
    expect(container.innerHTML).toBe('')
  })

  it('renders nothing when all values are falsy', () => {
    const { container } = render(
      <CapabilitiesSection capabilities={{ tools: null, resources: false, logging: undefined }} />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders known capability badges', () => {
    render(<CapabilitiesSection capabilities={{ tools: {}, resources: {}, logging: true }} />)
    expect(screen.getByText('Tools')).toBeInTheDocument()
    expect(screen.getByText('Resources')).toBeInTheDocument()
    expect(screen.getByText('Logging')).toBeInTheDocument()
  })

  it('renders unknown capability keys as-is', () => {
    render(<CapabilitiesSection capabilities={{ customCap: true }} />)
    expect(screen.getByText('customCap')).toBeInTheDocument()
  })

  it('renders expandable capabilities as buttons', () => {
    render(<CapabilitiesSection capabilities={{ tools: { listChanged: true } }} />)
    const btn = screen.getByRole('button', { name: /tools/i })
    expect(btn).toBeInTheDocument()
  })

  it('expands capability details on click', () => {
    render(<CapabilitiesSection capabilities={{ tools: { listChanged: true } }} />)
    const btn = screen.getByRole('button', { name: /tools/i })
    fireEvent.click(btn)
    expect(screen.getByText(/"listChanged": true/)).toBeInTheDocument()
  })

  it('collapses capability details on second click', () => {
    render(<CapabilitiesSection capabilities={{ tools: { listChanged: true } }} />)
    const btn = screen.getByRole('button', { name: /tools/i })
    fireEvent.click(btn)
    expect(screen.getByText(/"listChanged": true/)).toBeInTheDocument()
    fireEvent.click(btn)
    expect(screen.queryByText(/"listChanged": true/)).not.toBeInTheDocument()
  })

  it('shows Capabilities heading', () => {
    render(<CapabilitiesSection capabilities={{ tools: {} }} />)
    expect(screen.getByText('Capabilities')).toBeInTheDocument()
  })
})
