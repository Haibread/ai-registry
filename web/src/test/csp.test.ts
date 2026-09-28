// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (rel: string) => readFileSync(new URL(rel, import.meta.url), 'utf8')

function cspDirectives(): Map<string, string[]> {
  const match = read('../../nginx.conf').match(/add_header\s+Content-Security-Policy\s+"([^"]+)"/)
  if (!match) throw new Error('no Content-Security-Policy header in nginx.conf')
  return new Map(
    match[1]
      .split(';')
      .map((d) => d.trim().split(/\s+/))
      .filter((parts) => parts[0])
      .map(([name, ...sources]) => [name, sources]),
  )
}

describe('SPA Content-Security-Policy', () => {
  it('allows no inline script or eval', () => {
    const scriptSrc = cspDirectives().get('script-src')
    expect(scriptSrc).toBeDefined()
    expect(scriptSrc).not.toContain("'unsafe-inline'")
    expect(scriptSrc).not.toContain("'unsafe-eval'")
  })

  it('restricts connect-src to the same origin', () => {
    expect(cspDirectives().get('connect-src')).toEqual(["'self'"])
  })

  it('index.html has no inline script', () => {
    const scripts = [...read('../../index.html').matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/g)]
    expect(scripts.length).toBeGreaterThan(0)
    for (const [, attrs, body] of scripts) {
      expect(attrs).toMatch(/\bsrc=/)
      expect(body.trim()).toBe('')
    }
  })
})
