import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import { useRecordView, useRecordCopy } from './use-record-event'

const mockPOST = vi.fn().mockResolvedValue({})
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ POST: mockPOST }),
}))

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: qc }, children)
}

describe('useRecordView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fires a POST on mount', () => {
    renderHook(() => useRecordView('acme', 'test-server'), { wrapper: wrapper() })
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/view',
      { params: { path: { namespace: 'acme', slug: 'test-server' } } },
    )
  })

  it('does not fire when namespace is missing', () => {
    renderHook(() => useRecordView(undefined, 'test'), { wrapper: wrapper() })
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('does not fire when slug is missing', () => {
    renderHook(() => useRecordView('acme', undefined), { wrapper: wrapper() })
    expect(mockPOST).not.toHaveBeenCalled()
  })
})

describe('useRecordCopy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns a function that fires a POST', () => {
    const { result } = renderHook(() => useRecordCopy('acme', 'srv'), { wrapper: wrapper() })
    result.current()
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/copy',
      { params: { path: { namespace: 'acme', slug: 'srv' } } },
    )
  })

  it('does nothing when namespace is missing', () => {
    const { result } = renderHook(() => useRecordCopy(undefined, 'srv'), { wrapper: wrapper() })
    result.current()
    expect(mockPOST).not.toHaveBeenCalled()
  })
})
