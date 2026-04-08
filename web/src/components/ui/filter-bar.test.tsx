/**
 * filter-bar.test.tsx
 *
 * Unit tests for the FilterBar client component.
 *
 * next/navigation is mocked so the component can render in jsdom without a
 * real Next.js runtime. Because the component initialises its local state from
 * useSearchParams (the URL is the source of truth), tests set
 * mockSearchParamsString before rendering to simulate active filters.
 *
 * We verify:
 *  - Text inputs are pre-filled from the active URL params.
 *  - Select dropdowns render all provided options and reflect the URL param.
 *  - Visibility filter is hidden by default; shown when showVisibility=true.
 *  - Clear button appears only when at least one filter is active, and calls
 *    router.replace(pathname) when clicked.
 *  - Typing in a text input calls router.replace() only after the debounce.
 *  - Changing a select calls router.replace() immediately.
 *  - Cursor param is always stripped when any filter changes.
 */

import { render, screen, fireEvent, act } from "@testing-library/react"
import { describe, it, expect, vi, beforeEach } from "vitest"
import { FilterBar } from "./filter-bar"

// ── Mock next/navigation ──────────────────────────────────────────────────────

const mockReplace = vi.fn()
const PATHNAME = "/mcp"

// Tests mutate this string before rendering to control what useSearchParams
// returns. Each test starts from the beforeEach reset (empty string).
let mockSearchParamsString = ""

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace }),
  usePathname: () => PATHNAME,
  useSearchParams: () => new URLSearchParams(mockSearchParamsString),
}))

beforeEach(() => {
  mockReplace.mockClear()
  mockSearchParamsString = ""
  vi.useFakeTimers()
})

// ── Helpers ───────────────────────────────────────────────────────────────────

function typeInto(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } })
}

function flushDebounce() {
  act(() => { vi.advanceTimersByTime(350) })
}

// ── Rendering ─────────────────────────────────────────────────────────────────

describe("FilterBar — rendering", () => {
  it("pre-fills search input from URL param q", () => {
    mockSearchParamsString = "q=hello"
    render(<FilterBar statusOptions={[]} />)
    expect((screen.getByPlaceholderText("Search…") as HTMLInputElement).value).toBe("hello")
  })

  it("uses a custom searchPlaceholder", () => {
    render(<FilterBar searchPlaceholder="Search servers…" statusOptions={[]} />)
    expect(screen.getByPlaceholderText("Search servers…")).toBeInTheDocument()
  })

  it("pre-fills namespace input from URL param namespace", () => {
    mockSearchParamsString = "namespace=acme"
    render(<FilterBar statusOptions={[]} />)
    expect((screen.getByPlaceholderText("Publisher…") as HTMLInputElement).value).toBe("acme")
  })

  it("renders status options with title-cased labels", () => {
    render(<FilterBar statusOptions={["draft", "published", "deprecated"]} />)
    expect(screen.getByRole("option", { name: "All statuses" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "Draft" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "Published" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "Deprecated" })).toBeInTheDocument()
  })

  it("pre-selects status from URL param", () => {
    mockSearchParamsString = "status=published"
    render(<FilterBar statusOptions={["draft", "published", "deprecated"]} />)
    expect((screen.getByLabelText("Filter by status") as HTMLSelectElement).value).toBe("published")
  })

  it("hides the visibility filter by default", () => {
    render(<FilterBar statusOptions={[]} />)
    expect(screen.queryByLabelText("Filter by visibility")).not.toBeInTheDocument()
  })

  it("shows the visibility filter when showVisibility=true", () => {
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect(screen.getByLabelText("Filter by visibility")).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "All visibility" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "Public" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "Private" })).toBeInTheDocument()
  })

  it("pre-selects visibility from URL param", () => {
    mockSearchParamsString = "visibility=private"
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect((screen.getByLabelText("Filter by visibility") as HTMLSelectElement).value).toBe("private")
  })

  it("renders inside a <form> element", () => {
    const { container } = render(<FilterBar statusOptions={[]} />)
    expect(container.querySelector("form")).toBeTruthy()
  })
})

// ── Clear button ──────────────────────────────────────────────────────────────

describe("FilterBar — Clear button", () => {
  it("is always visible but disabled when no filters are active", () => {
    render(<FilterBar statusOptions={[]} />)
    const btn = screen.getByRole("button", { name: /clear/i })
    expect(btn).toBeInTheDocument()
    expect(btn).toBeDisabled()
  })

  it("is shown when URL param q is set", () => {
    mockSearchParamsString = "q=hello"
    render(<FilterBar statusOptions={[]} />)
    expect(screen.getByRole("button", { name: /clear/i })).toBeInTheDocument()
  })

  it("is shown when URL param namespace is set", () => {
    mockSearchParamsString = "namespace=acme"
    render(<FilterBar statusOptions={[]} />)
    expect(screen.getByRole("button", { name: /clear/i })).toBeInTheDocument()
  })

  it("is shown when URL param status is set", () => {
    mockSearchParamsString = "status=draft"
    render(<FilterBar statusOptions={["draft"]} />)
    expect(screen.getByRole("button", { name: /clear/i })).toBeInTheDocument()
  })

  it("is shown when URL param visibility is set", () => {
    mockSearchParamsString = "visibility=public"
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect(screen.getByRole("button", { name: /clear/i })).toBeInTheDocument()
  })

  it("calls router.replace(pathname) with no params when clicked", () => {
    mockSearchParamsString = "q=test"
    render(<FilterBar statusOptions={[]} />)
    fireEvent.click(screen.getByRole("button", { name: /clear/i }))
    expect(mockReplace).toHaveBeenCalledWith(PATHNAME)
  })
})

// ── Text input debounce ───────────────────────────────────────────────────────

describe("FilterBar — text input debounce", () => {
  it("does NOT call router.replace immediately on typing", () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText("Search…"), "foo")
    expect(mockReplace).not.toHaveBeenCalled()
  })

  it("calls router.replace after the debounce window", () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText("Search…"), "foo")
    flushDebounce()
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).toContain("q=foo")
  })

  it("debounces multiple keystrokes into a single navigation", () => {
    render(<FilterBar statusOptions={[]} />)
    const input = screen.getByPlaceholderText("Search…")
    typeInto(input, "f")
    act(() => { vi.advanceTimersByTime(100) })
    typeInto(input, "fo")
    act(() => { vi.advanceTimersByTime(100) })
    typeInto(input, "foo")
    flushDebounce()
    // Only the final value triggers a navigation.
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).toContain("q=foo")
  })

  it("removes q param from URL when input is cleared", () => {
    mockSearchParamsString = "q=hello"
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText("Search…"), "")
    flushDebounce()
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).not.toContain("q=")
  })

  it("namespace input also debounces", () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText("Publisher…"), "acme")
    expect(mockReplace).not.toHaveBeenCalled()
    flushDebounce()
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).toContain("namespace=acme")
  })
})

// ── Select immediate navigation ───────────────────────────────────────────────

describe("FilterBar — select immediate navigation", () => {
  it("calls router.replace immediately when status changes", () => {
    render(<FilterBar statusOptions={["draft", "published"]} />)
    fireEvent.change(screen.getByLabelText("Filter by status"), {
      target: { value: "published" },
    })
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).toContain("status=published")
  })

  it("removes status param when 'All statuses' is selected", () => {
    mockSearchParamsString = "status=draft"
    render(<FilterBar statusOptions={["draft", "published"]} />)
    fireEvent.change(screen.getByLabelText("Filter by status"), {
      target: { value: "" },
    })
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).not.toContain("status=")
  })

  it("calls router.replace immediately when visibility changes", () => {
    render(<FilterBar statusOptions={[]} showVisibility />)
    fireEvent.change(screen.getByLabelText("Filter by visibility"), {
      target: { value: "private" },
    })
    expect(mockReplace).toHaveBeenCalledOnce()
    expect(mockReplace.mock.calls[0][0]).toContain("visibility=private")
  })

  it("strips cursor param when a filter changes", () => {
    mockSearchParamsString = "cursor=abc123"
    render(<FilterBar statusOptions={["draft"]} />)
    fireEvent.change(screen.getByLabelText("Filter by status"), {
      target: { value: "draft" },
    })
    expect(mockReplace.mock.calls[0][0]).not.toContain("cursor=")
  })
})
