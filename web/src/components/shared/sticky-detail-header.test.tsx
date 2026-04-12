import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { StickyDetailHeader } from './sticky-detail-header'
import React from 'react'

// We can't easily test IntersectionObserver in jsdom, so we mock it.
let observerCallback: IntersectionObserverCallback
let mockObserve: ReturnType<typeof vi.fn>
let mockDisconnect: ReturnType<typeof vi.fn>

beforeEach(() => {
  mockObserve = vi.fn()
  mockDisconnect = vi.fn()
  ;(globalThis as any).IntersectionObserver = vi.fn((cb: IntersectionObserverCallback) => {
    observerCallback = cb
    return { observe: mockObserve, disconnect: mockDisconnect, unobserve: vi.fn() }
  })
})

afterEach(() => {
  delete (globalThis as any).IntersectionObserver
})

describe('StickyDetailHeader', () => {
  function renderHeader(overrides = {}) {
    const titleEl = document.createElement('h1')
    const ref = { current: titleEl } as React.RefObject<HTMLElement>
    return render(
      <StickyDetailHeader
        type="mcp-server"
        name="Test Server"
        version="1.2.3"
        identifier="acme/test-server"
        titleRef={ref}
        {...overrides}
      />,
    )
  }

  it('renders the name', () => {
    renderHeader()
    expect(screen.getByText('Test Server')).toBeInTheDocument()
  })

  it('renders the version badge', () => {
    renderHeader()
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('does not show version badge when version is undefined', () => {
    renderHeader({ version: undefined })
    expect(screen.queryByText(/^v/)).not.toBeInTheDocument()
  })

  it('starts hidden (pointer-events-none)', () => {
    const { container } = renderHeader()
    const header = container.firstChild as HTMLElement
    expect(header.className).toContain('pointer-events-none')
  })

  it('becomes visible when title scrolls out of view', () => {
    const { container } = renderHeader()
    // Simulate title leaving the viewport
    act(() => {
      observerCallback(
        [{ isIntersecting: false } as IntersectionObserverEntry],
        {} as IntersectionObserver,
      )
    })
    const header = container.firstChild as HTMLElement
    expect(header.className).not.toContain('pointer-events-none')
    expect(header.className).toContain('opacity-100')
  })

  it('hides again when title re-enters viewport', () => {
    const { container } = renderHeader()
    act(() => {
      observerCallback(
        [{ isIntersecting: false } as IntersectionObserverEntry],
        {} as IntersectionObserver,
      )
    })
    act(() => {
      observerCallback(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver,
      )
    })
    const header = container.firstChild as HTMLElement
    expect(header.className).toContain('pointer-events-none')
  })

  it('observes the title element', () => {
    const titleEl = document.createElement('h1')
    const ref = { current: titleEl } as React.RefObject<HTMLElement>
    render(
      <StickyDetailHeader
        type="mcp-server"
        name="X"
        identifier="a/b"
        titleRef={ref}
      />,
    )
    expect(mockObserve).toHaveBeenCalledWith(titleEl)
  })

  it('disconnects observer on unmount', () => {
    const { unmount } = renderHeader()
    unmount()
    expect(mockDisconnect).toHaveBeenCalled()
  })
})
