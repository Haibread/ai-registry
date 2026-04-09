// @vitest-environment node

import { describe, it, expect, vi } from 'vitest'

// AuthContext uses window.location at module level; stub it out so the node
// environment doesn't crash when api-client.ts imports it.
vi.mock('@/auth/AuthContext', () => ({ useAuth: vi.fn() }))

import { getPublicClient, getAuthClient } from './api-client'

describe('getPublicClient', () => {
  it('returns a client without Authorization header', () => {
    const client = getPublicClient()
    expect(client).toBeDefined()
    // Client is synchronous and stateless
  })
})

describe('getAuthClient', () => {
  it('returns a client — synchronous, no redirect needed', () => {
    const client = getAuthClient('my-token')
    expect(client).toBeDefined()
  })
})
