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
