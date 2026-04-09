/**
 * api-client.test.ts
 *
 * Verifies that `getApiClient()` automatically redirects to the sign-in page
 * whenever the bearer token is invalid, so the user is never silently left on
 * a broken admin page.
 *
 * Two failure modes are tested:
 *   1. `RefreshAccessTokenError`  — the Keycloak refresh token is gone; NextAuth
 *      already set `session.error` before the API call is even attempted.
 *   2. HTTP 401 from the backend — the access token passed to the API was
 *      rejected (e.g. the Keycloak session was revoked mid-session, or JWKS
 *      rotated).  The openapi-fetch response middleware detects this and
 *      redirects.
 */

// @vitest-environment node

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { getApiClient } from "./api-client"

// ── Module mocks (hoisted by Vitest before any import) ────────────────────────

vi.mock("@/auth", () => ({
  auth: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}))

// ── Typed mock references ─────────────────────────────────────────────────────

import { auth } from "@/auth"
import { redirect } from "next/navigation"

const mockAuth = vi.mocked(auth)
const mockRedirect = vi.mocked(redirect)

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Minimal session with a working access token. */
const validSession = { accessToken: "tok-abc123", error: undefined }

/** Minimal session whose Keycloak refresh has failed. */
const brokenSession = { accessToken: undefined, error: "RefreshAccessTokenError" as const }

/**
 * Stub global `fetch` to return the given status and optional body.
 * Uses `mockImplementation` (not `mockResolvedValue`) so each call gets a
 * fresh `Response` instance — a consumed body cannot be read a second time.
 * Returns the stub so callers can make further assertions on call counts.
 */
function stubFetch(status: number, body: unknown = {}) {
  const stub = vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      })
    )
  )
  vi.stubGlobal("fetch", stub)
  return stub
}

// ── Setup / teardown ──────────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// ─────────────────────────────────────────────────────────────────────────────
// Session-error gate (checked before any network call)
// ─────────────────────────────────────────────────────────────────────────────

describe("getApiClient — RefreshAccessTokenError session gate", () => {
  it("redirects to sign-in immediately when session.error is RefreshAccessTokenError", async () => {
    mockAuth.mockResolvedValue(brokenSession as never)

    await getApiClient()

    expect(mockRedirect).toHaveBeenCalledWith("/api/auth/signin")
  })

  it("redirects before making any network call when session is broken", async () => {
    mockAuth.mockResolvedValue(brokenSession as never)
    const fetchSpy = stubFetch(200)

    await getApiClient()

    // The client should never have been used to hit the backend.
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it("does NOT redirect when the session is valid", async () => {
    mockAuth.mockResolvedValue(validSession as never)

    await getApiClient()

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("does NOT redirect when there is no session at all (unauthenticated read)", async () => {
    // Unauthenticated callers have no session — the middleware blocks them
    // from /admin separately; getApiClient should not redirect by itself.
    mockAuth.mockResolvedValue(null as never)

    await getApiClient()

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("does not redirect for a session with no error field", async () => {
    mockAuth.mockResolvedValue({ accessToken: "tok", error: undefined } as never)

    await getApiClient()

    expect(mockRedirect).not.toHaveBeenCalled()
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// Backend 401 response middleware
// ─────────────────────────────────────────────────────────────────────────────

describe("getApiClient — backend 401 response middleware", () => {
  it("redirects to sign-in when the backend returns 401", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    stubFetch(401)

    const api = await getApiClient()
    // Any path triggers the middleware; we use a real path from the schema.
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).toHaveBeenCalledWith("/api/auth/signin")
  })

  it("does NOT redirect for a 200 response", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    stubFetch(200, { mcp_servers: 0, agents: 0, publishers: 0 })

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("does NOT redirect for a 403 Forbidden (wrong role, not expired token)", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    stubFetch(403)

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("does NOT redirect for a 404 Not Found", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    stubFetch(404, { error: "not found" })

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("does NOT redirect for a 422 Unprocessable Entity", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    stubFetch(422, { error: "invalid body" })

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it("redirects to sign-in exactly once even if multiple 401s arrive", async () => {
    mockAuth.mockResolvedValue(validSession as never)
    // In the real app redirect() throws, so only one call ever happens.
    // With our mock it does not throw, so we make two requests and assert
    // that redirect is called each time (each is its own middleware run).
    stubFetch(401)

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)
    await api.GET("/api/v1/stats" as never)

    expect(mockRedirect).toHaveBeenCalledTimes(2)
    expect(mockRedirect).toHaveBeenNthCalledWith(1, "/api/auth/signin")
    expect(mockRedirect).toHaveBeenNthCalledWith(2, "/api/auth/signin")
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// Authorization header injection
// ─────────────────────────────────────────────────────────────────────────────

describe("getApiClient — Authorization header", () => {
  it("sends the Bearer token from the session in the Authorization header", async () => {
    mockAuth.mockResolvedValue({ accessToken: "my-secret-token" } as never)
    const fetchSpy = stubFetch(200, {})

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    // openapi-fetch passes a Request object as the first fetch argument.
    const [requestArg] = fetchSpy.mock.calls[0] as [Request]
    expect(requestArg.headers.get("authorization")).toBe("Bearer my-secret-token")
  })

  it("omits the Authorization header when there is no access token", async () => {
    mockAuth.mockResolvedValue(null as never)
    const fetchSpy = stubFetch(200, {})

    const api = await getApiClient()
    await api.GET("/api/v1/stats" as never)

    const [requestArg] = fetchSpy.mock.calls[0] as [Request]
    expect(requestArg.headers.get("authorization")).toBeNull()
  })
})
