// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './AuthContext'
import { clearTokens, getRefreshToken, setTokens } from './tokens'

// AuthContext provides login/logout actions + the public
// sign-in feature flags from /config.json. Auth *state* lives in useMe, not here.

function Consumer() {
  const { oidcEnabled, localLoginEnabled, configLoading, loginLocal, logout, isAuthenticated } = useAuth()
  return (
    <div>
      <span data-testid="cfg">{configLoading ? 'loading' : `${oidcEnabled}:${localLoginEnabled}`}</span>
      <span data-testid="authed">{String(isAuthenticated)}</span>
      <button
        onClick={() =>
          loginLocal('a@b.com', 'pw').catch((e: Error) => {
            const el = document.getElementById('err')
            if (el) el.textContent = e.message
          })
        }
      >
        login
      </button>
      <button onClick={() => void logout()}>logout</button>
      <span id="err" />
    </div>
  )
}

function renderProvider() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <Consumer />
      </AuthProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.restoreAllMocks()
  clearTokens()
})

describe('AuthProvider', () => {
  it('loads sign-in feature flags from /config.json', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ oidc_enabled: true, local_login_enabled: false }), { status: 200 }),
    )
    renderProvider()
    await waitFor(() => expect(screen.getByTestId('cfg').textContent).toBe('true:false'))
  })

  it('defaults to local-login-on when /config.json fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('nope', { status: 500 }))
    renderProvider()
    await waitFor(() => expect(screen.getByTestId('cfg').textContent).toBe('false:true'))
  })

  it('loginLocal POSTs to /api/v1/auth/login (bearer, no cookie) and surfaces a 401', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      // On mount AuthProvider fetches /config.json then /api/v1/me.
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ oidc_enabled: false, local_login_enabled: true }), { status: 200 }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 401 })) // /me → signed-out
      .mockResolvedValueOnce(new Response(null, { status: 401 })) // login → error
    renderProvider()
    await waitFor(() => expect(screen.getByTestId('cfg').textContent).toBe('false:true'))

    await userEvent.click(screen.getByText('login'))

    await waitFor(() => expect(document.getElementById('err')?.textContent).toBe('Invalid email or password.'))
    // Bearer model: the login POST carries a JSON body and no cookie credentials.
    const loginCall = fetchMock.mock.calls.find((c) => c[0] === '/api/v1/auth/login')
    expect(loginCall).toBeTruthy()
    expect(loginCall?.[1]).toMatchObject({ method: 'POST' })
    expect((loginCall?.[1] as RequestInit | undefined)?.credentials).toBeUndefined()
  })

  it('loginLocal stores the token pair and resolves identity on success', async () => {
    clearTokens()
    vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
      const url = String(input)
      if (url === '/config.json') {
        return Promise.resolve(
          new Response(JSON.stringify({ oidc_enabled: false, local_login_enabled: true }), { status: 200 }),
        )
      }
      if (url === '/api/v1/auth/login') {
        return Promise.resolve(
          new Response(JSON.stringify({ accessToken: 'acc', refreshToken: 'ref', tokenType: 'Bearer', expiresIn: 900 }), {
            status: 200,
          }),
        )
      }
      if (url === '/api/v1/me') {
        return Promise.resolve(new Response(JSON.stringify({ authenticated: true, grants: [] }), { status: 200 }))
      }
      return Promise.resolve(new Response(null, { status: 404 }))
    })
    renderProvider()
    await waitFor(() => expect(screen.getByTestId('cfg').textContent).toBe('false:true'))

    await userEvent.click(screen.getByText('login'))

    await waitFor(() => expect(getRefreshToken()).toBe('ref'))
  })

  it('consumes an OIDC handoff code from the URL fragment on mount', async () => {
    clearTokens()
    window.location.hash = '#code=handoff-xyz'
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
      const url = String(input)
      if (url === '/config.json') {
        return Promise.resolve(
          new Response(JSON.stringify({ oidc_enabled: true, local_login_enabled: true }), { status: 200 }),
        )
      }
      if (url === '/api/v1/auth/oidc/exchange') {
        return Promise.resolve(
          new Response(JSON.stringify({ accessToken: 'acc', refreshToken: 'oidc-ref', expiresIn: 900 }), { status: 200 }),
        )
      }
      if (url === '/api/v1/me') {
        return Promise.resolve(new Response(JSON.stringify({ authenticated: true, grants: [] }), { status: 200 }))
      }
      return Promise.resolve(new Response(null, { status: 404 }))
    })
    renderProvider()

    // The handoff code is exchanged for tokens and the fragment is stripped.
    await waitFor(() => expect(getRefreshToken()).toBe('oidc-ref'))
    expect(window.location.hash).toBe('')
    const exchangeCall = fetchMock.mock.calls.find((c) => c[0] === '/api/v1/auth/oidc/exchange')
    expect(exchangeCall).toBeTruthy()
    expect(JSON.parse((exchangeCall?.[1] as RequestInit).body as string)).toEqual({ code: 'handoff-xyz' })
  })

  it('logout revokes the refresh token and clears local tokens', async () => {
    clearTokens()
    setTokens('acc', 'ref')
    let logoutBody: unknown = null
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = String(input)
      if (url === '/config.json') {
        return Promise.resolve(
          new Response(JSON.stringify({ oidc_enabled: false, local_login_enabled: true }), { status: 200 }),
        )
      }
      if (url === '/api/v1/auth/refresh') {
        return Promise.resolve(
          new Response(JSON.stringify({ accessToken: 'acc2', refreshToken: 'ref2' }), { status: 200 }),
        )
      }
      if (url === '/api/v1/me') {
        return Promise.resolve(new Response(JSON.stringify({ authenticated: true, grants: [] }), { status: 200 }))
      }
      if (url === '/api/v1/auth/logout') {
        logoutBody = JSON.parse((init as RequestInit).body as string)
        return Promise.resolve(new Response(JSON.stringify({}), { status: 200 })) // local session: no oidcLogoutUrl
      }
      return Promise.resolve(new Response(null, { status: 404 }))
    })
    renderProvider()
    await waitFor(() => expect(screen.getByTestId('authed').textContent).toBe('true'))

    await userEvent.click(screen.getByText('logout'))

    await waitFor(() => expect(getRefreshToken()).toBeNull())
    // The revoke call carried the stored refresh token (mount reused the stored
    // access token, so it never refreshed/rotated).
    expect((logoutBody as { refreshToken?: string }).refreshToken).toBe('ref')
  })
})
