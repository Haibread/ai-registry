// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AuthProvider, useAuth, resetManagerForTesting } from './AuthContext'

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeUserManager(overrides: Partial<{
  signinRedirect: () => Promise<void>
  getUser: () => Promise<null>
}> = {}) {
  return {
    signinRedirect: vi.fn().mockResolvedValue(undefined),
    signoutRedirect: vi.fn().mockResolvedValue(undefined),
    removeUser: vi.fn().mockResolvedValue(undefined),
    getUser: vi.fn().mockResolvedValue(null),
    events: {
      addUserLoaded: vi.fn(),
      addUserUnloaded: vi.fn(),
      removeUserLoaded: vi.fn(),
      removeUserUnloaded: vi.fn(),
    },
    ...overrides,
  }
}

vi.mock('oidc-client-ts', () => ({
  UserManager: vi.fn(),
  // WebStorageStateStore is instantiated during UserManager creation.
  // Provide a no-op mock so jsdom's limited localStorage doesn't throw.
  WebStorageStateStore: vi.fn().mockImplementation(() => ({})),
}))

import { UserManager } from 'oidc-client-ts'
const MockUserManager = vi.mocked(UserManager)

function AuthConsumer() {
  const { isLoading, loginError, accessToken, login } = useAuth()
  return (
    <div>
      <span data-testid="loading">{String(isLoading)}</span>
      <span data-testid="error">{loginError ?? ''}</span>
      <span data-testid="token">{accessToken ?? ''}</span>
      <button onClick={login}>Sign in</button>
    </div>
  )
}

function renderAuth() {
  return render(
    <AuthProvider>
      <AuthConsumer />
    </AuthProvider>
  )
}

function mockConfigJson(um: ReturnType<typeof makeUserManager>) {
  MockUserManager.mockImplementation(() => um as never)
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    new Response(
      JSON.stringify({ oidc_issuer: 'https://auth.example.com', oidc_client_id: 'spa' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ),
  )
}

beforeEach(() => {
  // Reset the module-level promise cache so each test starts fresh.
  resetManagerForTesting()
  MockUserManager.mockReset()
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('AuthProvider — config.json failure', () => {
  it('sets loginError and resolves isLoading when /config.json returns non-200', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response('Not Found', { status: 404 }),
    )

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )
    expect(screen.getByTestId('error').textContent).toMatch(/Authentication configuration failed/)
    expect(screen.getByTestId('error').textContent).toMatch(/404/)
  })

  it('sets loginError and resolves isLoading when /config.json fetch throws (network error)', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('Failed to fetch'))

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )
    expect(screen.getByTestId('error').textContent).toMatch(/Authentication configuration failed/)
  })
})

describe('AuthProvider — login() with no UserManager', () => {
  it('shows a configuration error when login() is called before UserManager is ready', async () => {
    // fetch never resolves → um stays null
    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(new Promise(() => {}))

    renderAuth()

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /sign in/i }))
    })

    expect(screen.getByTestId('error').textContent).toMatch(/not configured/)
    expect(screen.getByTestId('error').textContent).toMatch(/config\.json/)
  })

  it('shows a configuration error when config.json failed and user clicks Sign In', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response('Not Found', { status: 404 }),
    )

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /sign in/i }))
    })

    expect(screen.getByTestId('error').textContent).toMatch(/not configured/)
  })
})

describe('AuthProvider — event wiring and session lifecycle', () => {
  // `oidc-client-ts` drives token refresh by firing `addUserLoaded` whenever
  // a new access token lands (initial sign-in, silent renew). Logout and
  // expired-session paths fire `addUserUnloaded`. The tests below capture
  // the callbacks the provider registers and invoke them directly — that
  // proves the handlers are wired correctly without having to mock the
  // entire oidc-client-ts renewal state machine.

  it('exposes the new access token when oidc-client-ts fires addUserLoaded (silent renew path)', async () => {
    let loadedCallback: ((u: { access_token: string }) => void) | undefined
    const um = makeUserManager()
    um.events.addUserLoaded = vi.fn((cb: (u: { access_token: string }) => void) => {
      loadedCallback = cb
    }) as never
    mockConfigJson(um)

    renderAuth()

    // Wait for the provider to finish its initial getUser() → isLoading=false.
    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )
    expect(loadedCallback).toBeTypeOf('function')
    expect(screen.getByTestId('token').textContent).toBe('')

    // Simulate a silent renew completing: oidc-client-ts calls the callback
    // we registered in useEffect.
    act(() => {
      loadedCallback!({ access_token: 'refreshed-token-v2' })
    })

    await waitFor(() =>
      expect(screen.getByTestId('token').textContent).toBe('refreshed-token-v2'),
    )
  })

  it('clears the access token when oidc-client-ts fires addUserUnloaded (expired session / logout)', async () => {
    let loadedCallback: ((u: { access_token: string }) => void) | undefined
    let unloadedCallback: (() => void) | undefined
    const um = makeUserManager()
    um.events.addUserLoaded = vi.fn((cb: (u: { access_token: string }) => void) => {
      loadedCallback = cb
    }) as never
    um.events.addUserUnloaded = vi.fn((cb: () => void) => {
      unloadedCallback = cb
    }) as never
    mockConfigJson(um)

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    // Seed a "logged-in" state first so the clear is observable.
    act(() => {
      loadedCallback!({ access_token: 'token-before-expiry' })
    })
    await waitFor(() =>
      expect(screen.getByTestId('token').textContent).toBe('token-before-expiry'),
    )

    // Now the session expires / user logs out — oidc-client-ts fires
    // addUserUnloaded. The provider must drop user state so RequireAuth
    // re-redirects and consumers stop sending stale Bearer tokens.
    act(() => {
      unloadedCallback!()
    })
    await waitFor(() =>
      expect(screen.getByTestId('token').textContent).toBe(''),
    )
  })

  it('unsubscribes from both events on unmount (no leaked handlers between remounts)', async () => {
    const um = makeUserManager()
    mockConfigJson(um)

    const { unmount } = renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    // Both add* were called exactly once during the effect.
    expect(um.events.addUserLoaded).toHaveBeenCalledOnce()
    expect(um.events.addUserUnloaded).toHaveBeenCalledOnce()

    // Unmount runs the effect cleanup, which must remove the handlers.
    unmount()
    expect(um.events.removeUserLoaded).toHaveBeenCalledOnce()
    expect(um.events.removeUserUnloaded).toHaveBeenCalledOnce()

    // And crucially, both remove* received the SAME callbacks that add*
    // received — a common subtle bug is to pass a fresh arrow function on
    // cleanup, which is a silent no-op at the oidc-client-ts level.
    const loadedAdded = (um.events.addUserLoaded as unknown as { mock: { calls: unknown[][] } }).mock.calls[0][0]
    const loadedRemoved = (um.events.removeUserLoaded as unknown as { mock: { calls: unknown[][] } }).mock.calls[0][0]
    expect(loadedRemoved).toBe(loadedAdded)
  })

  it('hydrates the initial token from UserManager.getUser() when a session is already in storage', async () => {
    const um = makeUserManager({
      getUser: vi.fn().mockResolvedValue({ access_token: 'persisted-token' } as never),
    })
    mockConfigJson(um)

    renderAuth()

    // The provider's Step-2 effect calls getUser() and stashes the result;
    // the derived accessToken in context reflects it once the promise resolves.
    await waitFor(() =>
      expect(screen.getByTestId('token').textContent).toBe('persisted-token'),
    )
    expect(screen.getByTestId('loading').textContent).toBe('false')
  })
})

describe('AuthProvider — login() with UserManager ready', () => {
  it('calls signinRedirect when UserManager is ready', async () => {
    const um = makeUserManager()
    mockConfigJson(um)

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /sign in/i }))
    })

    expect(um.signinRedirect).toHaveBeenCalledOnce()
    expect(screen.getByTestId('error').textContent).toBe('')
  })

  it('shows CORS/network error when signinRedirect fails with Failed to fetch', async () => {
    const um = makeUserManager({
      signinRedirect: vi.fn().mockRejectedValue(new TypeError('Failed to fetch')),
    })
    mockConfigJson(um)

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /sign in/i }))
    })

    await waitFor(() =>
      expect(screen.getByTestId('error').textContent).toMatch(/Cannot reach the authentication server/),
    )
  })

  it('shows generic error when signinRedirect fails with an unexpected error', async () => {
    const um = makeUserManager({
      signinRedirect: vi.fn().mockRejectedValue(new Error('invalid_client')),
    })
    mockConfigJson(um)

    renderAuth()

    await waitFor(() =>
      expect(screen.getByTestId('loading').textContent).toBe('false'),
    )

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /sign in/i }))
    })

    await waitFor(() =>
      expect(screen.getByTestId('error').textContent).toMatch(/Sign-in failed.*invalid_client/),
    )
  })
})
