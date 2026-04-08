/**
 * auto-sign-out.test.tsx
 *
 * Verifies that <AutoSignOut> calls `signOut` from next-auth/react immediately
 * on mount and shows a user-facing "session expired" message while it does so.
 *
 * This component is the last resort that actually clears the session cookie.
 * A plain server-side `redirect()` cannot mutate cookies and would leave the
 * broken session in place, potentially causing a redirect loop.  We verify
 * that the *client-side* signOut is invoked with the expected options.
 */

import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import { AutoSignOut } from "./auto-sign-out"

// ── Module mock ───────────────────────────────────────────────────────────────

vi.mock("next-auth/react", () => ({
  signOut: vi.fn(),
}))

import { signOut } from "next-auth/react"

const mockSignOut = vi.mocked(signOut)

// ── Setup ─────────────────────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
})

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

describe("<AutoSignOut> — rendering", () => {
  it("renders a user-facing 'session expired' message", () => {
    render(<AutoSignOut />)
    expect(screen.getByText(/session expired/i)).toBeInTheDocument()
  })

  it("mentions 'signing you out' in the message so the user understands what is happening", () => {
    render(<AutoSignOut />)
    expect(screen.getByText(/signing you out/i)).toBeInTheDocument()
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// Sign-out behaviour
// ─────────────────────────────────────────────────────────────────────────────

describe("<AutoSignOut> — signOut invocation", () => {
  it("calls signOut on mount", () => {
    render(<AutoSignOut />)
    expect(mockSignOut).toHaveBeenCalledTimes(1)
  })

  it("redirects to '/' after sign-out so the user lands on the public home page", () => {
    render(<AutoSignOut />)
    expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" })
  })

  it("does NOT call signOut more than once on re-renders (useEffect dep array is empty)", () => {
    const { rerender } = render(<AutoSignOut />)
    rerender(<AutoSignOut />)
    rerender(<AutoSignOut />)
    expect(mockSignOut).toHaveBeenCalledTimes(1)
  })
})
