import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { InstallCommand } from './install-command'

describe('InstallCommand', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    writeText.mockClear()
    Object.assign(navigator, { clipboard: { writeText } })
  })

  it('renders the command text', () => {
    render(<InstallCommand command="npm install @acme/files" />)
    expect(screen.getByText('npm install @acme/files')).toBeInTheDocument()
  })

  it('renders a Copy command button', () => {
    render(<InstallCommand command="npx foo" />)
    expect(screen.getByRole('button', { name: /copy command/i })).toBeInTheDocument()
  })

  it('copies the command to the clipboard when clicked', async () => {
    render(<InstallCommand command="npx foo" />)
    fireEvent.click(screen.getByRole('button', { name: /copy command/i }))
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('npx foo')
    })
  })

  it('fires onCopy after a successful copy', async () => {
    const onCopy = vi.fn()
    render(<InstallCommand command="pip install x" onCopy={onCopy} />)
    fireEvent.click(screen.getByRole('button', { name: /copy command/i }))
    await waitFor(() => {
      expect(onCopy).toHaveBeenCalled()
    })
  })

  it('applies custom className', () => {
    const { container } = render(
      <InstallCommand command="echo hi" className="custom-cls" />,
    )
    expect(container.firstChild).toHaveClass('custom-cls')
  })
})
