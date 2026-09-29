import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MCPCodeSnippets } from './code-snippets'

const npmPackage = {
  registryType: 'npm',
  identifier: '@acme/test-server',
  version: '1.0.0',
  transport: { type: 'stdio' },
}
const sseRemote = { type: 'sse', url: 'https://mcp.example.com/sse' }
const tools = [{ name: 'search_docs' }]

describe('MCPCodeSnippets', () => {
  it('renders nothing without packages or remotes', () => {
    const { container } = render(<MCPCodeSnippets serverName="test" packages={[]} />)
    expect(container.innerHTML).toBe('')
  })

  it('shows the Python snippet by default', () => {
    render(<MCPCodeSnippets serverName="test" packages={[npmPackage]} tools={tools} />)
    expect(screen.getByRole('button', { name: 'Python' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText(/pip install "mcp==2\.2\.0"/)).toBeInTheDocument()
    expect(screen.getByText(/StdioServerParameters\(command="npx"/)).toBeInTheDocument()
    expect(screen.getByText(/call_tool\("search_docs"/)).toBeInTheDocument()
  })

  it('switches language', async () => {
    const user = userEvent.setup()
    render(<MCPCodeSnippets serverName="test" packages={[npmPackage]} />)
    await user.click(screen.getByRole('button', { name: 'Go' }))
    expect(screen.getByText(/go get github\.com\/modelcontextprotocol\/go-sdk@v1\.8\.0/)).toBeInTheDocument()
    expect(screen.getByText(/mcp\.CommandTransport/)).toBeInTheDocument()
  })

  it('offers a connection selector only when there are several sources', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<MCPCodeSnippets serverName="test" packages={[npmPackage]} />)
    expect(screen.queryByRole('combobox', { name: /select code connection/i })).not.toBeInTheDocument()

    rerender(<MCPCodeSnippets serverName="test" packages={[npmPackage]} remotes={[sseRemote]} />)
    await user.selectOptions(screen.getByRole('combobox', { name: /select code connection/i }), '1')
    expect(screen.getByText(/sse_client\("https:\/\/mcp\.example\.com\/sse"\)/)).toBeInTheDocument()
  })

  it('explains that Rust cannot reach an SSE-only server', async () => {
    const user = userEvent.setup()
    render(<MCPCodeSnippets serverName="test" packages={[]} remotes={[sseRemote]} />)
    await user.click(screen.getByRole('button', { name: 'Rust' }))
    expect(screen.getByText(/no client for the legacy SSE transport/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy code' })).not.toBeInTheDocument()
  })
})
