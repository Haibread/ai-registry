import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { VersionDiff } from './version-diff'

describe('VersionDiff', () => {
  it('shows a "no differences" message when versions are equivalent', () => {
    render(
      <VersionDiff
        a={{ version: '1.0.0', runtime: 'node', protocol_version: '2025-03-26' }}
        b={{ version: '1.0.1', runtime: 'node', protocol_version: '2025-03-26' }}
      />,
    )
    expect(screen.getByText(/no differences in structured fields/i)).toBeInTheDocument()
  })

  it('renders old → new for changed scalar fields', () => {
    render(
      <VersionDiff
        a={{ version: '1.0.0', runtime: 'node', protocol_version: '2025-03-26' }}
        b={{ version: '1.0.1', runtime: 'python', protocol_version: '2025-03-26' }}
      />,
    )
    expect(screen.getByTestId('diff-field-list')).toBeInTheDocument()
    expect(screen.getByText('node')).toBeInTheDocument()
    expect(screen.getByText('python')).toBeInTheDocument()
  })

  it('stringifies nested object differences', () => {
    render(
      <VersionDiff
        a={{ version: '1.0.0', packages: [{ name: 'a' }] }}
        b={{ version: '1.0.1', packages: [{ name: 'b' }] }}
      />,
    )
    const list = screen.getByTestId('diff-field-list')
    expect(list.textContent).toContain('packages')
    expect(list.textContent).toContain('"name": "a"')
    expect(list.textContent).toContain('"name": "b"')
  })

  it('shows the version labels in the header', () => {
    render(<VersionDiff a={{ version: '1.0.0' }} b={{ version: '2.0.0' }} />)
    expect(screen.getByText('v1.0.0')).toBeInTheDocument()
    expect(screen.getByText('v2.0.0')).toBeInTheDocument()
  })
})
