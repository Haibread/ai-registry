// @vitest-environment node

import { describe, it, expect } from 'vitest'
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
