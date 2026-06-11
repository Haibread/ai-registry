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

  it('shows connection selector when multiple packages', () => {
    const packages = [
      npmPackage,
      { registryType: 'pypi', identifier: 'test-py', version: '2.0.0', transport: { type: 'stdio' as const } },
    ]
    render(<MCPConfigGenerator serverName="test" packages={packages} />)
    expect(screen.getByRole('combobox', { name: /select connection/i })).toBeInTheDocument()
  })

  it('does not show connection selector for single package', () => {
    render(<MCPConfigGenerator serverName="test" packages={[npmPackage]} />)
    expect(screen.queryByRole('combobox', { name: /select connection/i })).not.toBeInTheDocument()
  })

  it('generates a url config from a remote endpoint with no packages', () => {
    render(
      <MCPConfigGenerator
        serverName="test"
        packages={[]}
        remotes={[{ type: 'sse', url: 'https://mcp.example.com/sse' }]}
      />,
    )
    expect(screen.getByText(/https:\/\/mcp\.example\.com\/sse/)).toBeInTheDocument()
    expect(screen.queryByText(/npx/)).not.toBeInTheDocument()
  })

  it('lists both packages and remotes in the connection selector', () => {
    render(
      <MCPConfigGenerator
        serverName="test"
        packages={[npmPackage]}
        remotes={[{ type: 'streamable_http', url: 'https://mcp.example.com/mcp' }]}
      />,
    )
    const select = screen.getByRole('combobox', { name: /select connection/i })
    expect(select).toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: /@acme\/test-server \(stdio\)/i }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: /https:\/\/mcp\.example\.com\/mcp \(streamable_http\)/i }),
    ).toBeInTheDocument()
  })

  it('renders nothing when both packages and remotes are empty', () => {
    const { container } = render(
      <MCPConfigGenerator serverName="test" packages={[]} remotes={[]} />,
    )
    expect(container.innerHTML).toBe('')
  })
})
