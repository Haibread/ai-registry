import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useRecordView, useRecordCopy } from './use-record-event'

const mockPOST = vi.fn().mockResolvedValue({})
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ POST: mockPOST }),
}))

describe('useRecordView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fires a POST on mount for mcp type', () => {
    renderHook(() => useRecordView('mcp', 'acme', 'test-server'))
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/view',
      { params: { path: { namespace: 'acme', slug: 'test-server' } } },
    )
  })

  it('fires a POST on mount for agent type', () => {
    renderHook(() => useRecordView('agent', 'acme', 'bot'))
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/agents/{namespace}/{slug}/view',
      { params: { path: { namespace: 'acme', slug: 'bot' } } },
    )
  })

  it('does not fire when namespace is missing', () => {
    renderHook(() => useRecordView('mcp', undefined, 'test'))
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('does not fire when slug is missing', () => {
    renderHook(() => useRecordView('mcp', 'acme', undefined))
    expect(mockPOST).not.toHaveBeenCalled()
  })
})

describe('useRecordCopy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns a function that fires a POST for mcp type', () => {
    const { result } = renderHook(() => useRecordCopy('mcp', 'acme', 'srv'))
    result.current()
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/copy',
      { params: { path: { namespace: 'acme', slug: 'srv' } } },
    )
  })

  it('returns a function that fires a POST for agent type', () => {
    const { result } = renderHook(() => useRecordCopy('agent', 'acme', 'bot'))
    result.current()
    expect(mockPOST).toHaveBeenCalledWith(
      '/api/v1/agents/{namespace}/{slug}/copy',
      { params: { path: { namespace: 'acme', slug: 'bot' } } },
    )
  })

  it('does nothing when namespace is missing', () => {
    const { result } = renderHook(() => useRecordCopy('mcp', undefined, 'srv'))
    result.current()
    expect(mockPOST).not.toHaveBeenCalled()
  })
})
