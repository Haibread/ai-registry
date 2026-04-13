import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MCPConfigGenerator } from './config-generator'

const npmPackage = {
  registryType: 'npm',
  identifier: '@acme/test-server',
  version: '1.0.0',
  transport: { type: 'stdio' as const },
}

describe('MCPConfigGenerator', () => {
  it('renders nothing when packages is empty', () => {
    const { container } = render(<MCPConfigGenerator serverName="test" packages={[]} />)
    expect(container.innerHTML).toBe('')
  })

  it('renders host selector', () => {
    render(<MCPConfigGenerator serverName="test" packages={[npmPackage]} />)
    expect(screen.getByRole('combobox', { name: /select mcp host/i })).toBeInTheDocument()
  })

  it('shows generated JSON config', () => {
    render(<MCPConfigGenerator serverName="test" packages={[npmPackage]} />)
    expect(screen.getByText(/npx/)).toBeInTheDocument()
  })

  it('changes config when host changes', async () => {
    const user = userEvent.setup()
    render(<MCPConfigGenerator serverName="test" packages={[npmPackage]} />)
    const select = screen.getByRole('combobox', { name: /select mcp host/i })
    // Switch to VS Code
    await user.selectOptions(select, '4') // VS Code is index 4
    expect(screen.getByText(/\.vscode\/mcp\.json/)).toBeInTheDocument()
  })

  it('shows package selector when multiple packages', () => {
    const packages = [
      npmPackage,
      { registryType: 'pypi', identifier: 'test-py', version: '2.0.0', transport: { type: 'stdio' as const } },
    ]
    render(<MCPConfigGenerator serverName="test" packages={packages} />)
    expect(screen.getByRole('combobox', { name: /select package/i })).toBeInTheDocument()
  })

  it('does not show package selector for single package', () => {
    render(<MCPConfigGenerator serverName="test" packages={[npmPackage]} />)
    expect(screen.queryByRole('combobox', { name: /select package/i })).not.toBeInTheDocument()
  })
})
