// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './AuthContext'

// AuthContext (ADR 0006 amendment) provides login/logout actions + the public
// sign-in feature flags from /config.json. Auth *state* lives in useMe, not here.

function Consumer() {
  const { oidcEnabled, localLoginEnabled, configLoading, loginLocal } = useAuth()
  return (
    <div>
      <span data-testid="cfg">{configLoading ? 'loading' : `${oidcEnabled}:${localLoginEnabled}`}</span>
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

  it('loginLocal POSTs to /api/v1/auth/login (with credentials) and surfaces a 401', async () => {
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
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({ method: 'POST', credentials: 'include' }),
    )
  })
})
