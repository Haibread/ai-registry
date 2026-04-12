import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AgentSnippetGenerator } from './snippet-generator'

describe('AgentSnippetGenerator', () => {
  it('renders language tabs', () => {
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    expect(screen.getByRole('button', { name: /curl/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /python/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /typescript/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /go/i })).toBeInTheDocument()
  })

  it('shows curl snippet by default', () => {
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    expect(screen.getByText(/curl -X POST/)).toBeInTheDocument()
  })

  it('includes endpoint URL in snippet', () => {
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    expect(screen.getByText(/agent\.example\.com/)).toBeInTheDocument()
  })

  it('switches to Python snippet on tab click', async () => {
    const user = userEvent.setup()
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    await user.click(screen.getByRole('button', { name: /python/i }))
    expect(screen.getByText(/import httpx/)).toBeInTheDocument()
  })

  it('switches to TypeScript snippet', async () => {
    const user = userEvent.setup()
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    await user.click(screen.getByRole('button', { name: /typescript/i }))
    expect(screen.getByText(/await fetch/)).toBeInTheDocument()
  })

  it('uses Bearer token by default', () => {
    render(<AgentSnippetGenerator endpointUrl="https://agent.example.com/a2a" />)
    expect(screen.getByText(/Bearer YOUR_TOKEN/)).toBeInTheDocument()
  })

  it('uses ApiKey when auth scheme is apikey', () => {
    render(
      <AgentSnippetGenerator
        endpointUrl="https://agent.example.com/a2a"
        authSchemes={['ApiKey']}
      />,
    )
    expect(screen.getByText(/ApiKey YOUR_API_KEY/)).toBeInTheDocument()
  })
})
