import { describe, it, expect } from 'vitest'
import { getModeInfo, getModeLabel } from './mode-labels'

describe('getModeInfo', () => {
  it('returns info for known mode text/plain', () => {
    const info = getModeInfo('text/plain')
    expect(info).toBeDefined()
    expect(info!.label).toBe('Plain text')
    expect(info!.description).toBeTruthy()
  })

  it('returns info for text/markdown', () => {
    const info = getModeInfo('text/markdown')
    expect(info).toBeDefined()
    expect(info!.label).toBe('Markdown')
  })

  it('returns info for application/json', () => {
    const info = getModeInfo('application/json')
    expect(info).toBeDefined()
    expect(info!.label).toBe('JSON')
  })

  it('returns info for image/*', () => {
    const info = getModeInfo('image/*')
    expect(info).toBeDefined()
    expect(info!.label).toBe('Images')
  })

  it('returns undefined for unknown modes', () => {
    expect(getModeInfo('application/xml')).toBeUndefined()
    expect(getModeInfo('foo/bar')).toBeUndefined()
  })
})

describe('getModeLabel', () => {
  it('returns human label for known mode', () => {
    expect(getModeLabel('text/plain')).toBe('Plain text')
    expect(getModeLabel('application/pdf')).toBe('PDF')
  })

  it('returns raw mode string for unknown modes', () => {
    expect(getModeLabel('application/xml')).toBe('application/xml')
    expect(getModeLabel('custom/type')).toBe('custom/type')
  })
})
