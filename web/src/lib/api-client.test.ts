// @vitest-environment node

import { describe, it, expect } from 'vitest'
import { getPublicClient } from './api-client'

describe('getPublicClient', () => {
  it('returns a client', () => {
    expect(getPublicClient()).toBeDefined()
  })
})
