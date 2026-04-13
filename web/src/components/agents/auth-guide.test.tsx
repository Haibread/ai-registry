import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthGuide } from './auth-guide'

describe('AuthGuide', () => {
  it('renders nothing when schemes is empty', () => {
    const { container } = render(<AuthGuide schemes={[]} />)
    expect(container.innerHTML).toBe('')
  })

  it('shows Bearer guide for bearer scheme', () => {
    render(<AuthGuide schemes={[{ scheme: 'Bearer' }]} />)
    // Title is "Bearer Token" — use exact match on heading text
    expect(screen.getByText('Bearer Token')).toBeInTheDocument()
  })

  it('shows OAuth2 guide for oauth2 scheme', () => {
    render(<AuthGuide schemes={[{ scheme: 'OAuth2' }]} />)
    expect(screen.getByText('OAuth 2.0')).toBeInTheDocument()
  })

  it('shows OpenID Connect guide', () => {
    render(<AuthGuide schemes={[{ scheme: 'OpenIdConnect' }]} />)
    expect(screen.getByText('OpenID Connect')).toBeInTheDocument()
  })

  it('shows API Key guide', () => {
    render(<AuthGuide schemes={[{ scheme: 'ApiKey' }]} />)
    expect(screen.getByText('API Key')).toBeInTheDocument()
  })

  it('shows multiple schemes', () => {
    render(<AuthGuide schemes={[{ scheme: 'Bearer' }, { scheme: 'ApiKey' }]} />)
    expect(screen.getByText('Bearer Token')).toBeInTheDocument()
    expect(screen.getByText('API Key')).toBeInTheDocument()
  })
})
