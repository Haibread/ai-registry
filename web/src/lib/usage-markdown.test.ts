import { describe, expect, it } from 'vitest'
import {
  applyMarkdownFormat,
  insertText,
  renderUsageMarkdown,
  starterUsageMarkdown,
  usageValues,
} from '@/lib/usage-markdown'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

const base: MCPServer = {
  id: 'srv-1',
  namespace: 'acme',
  slug: 'github-tools',
  name: 'GitHub Tools',
  visibility: 'public',
  status: 'published',
  featured: false,
  verified: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

describe('usageValues', () => {
  it('derives the run command and endpoint from the latest version', () => {
    const values = usageValues({
      ...base,
      latest_version: {
        version: '1.4.2',
        runtime: 'stdio',
        protocol_versions: ['2025-06-18'],
        packages: [
          { registryType: 'npm', identifier: '@acme/github-tools', version: '1.4.2', transport: { type: 'stdio' } },
        ],
        remotes: [{ type: 'streamable_http', url: 'https://mcp.acme.dev' }],
      },
    })
    expect(values).toMatchObject({
      name: 'GitHub Tools',
      namespace: 'acme',
      version: '1.4.2',
      run_command: 'npx -y @acme/github-tools',
      endpoint_url: 'https://mcp.acme.dev',
    })
  })

  it('leaves version-derived values unset without a published version', () => {
    const values = usageValues(base)
    expect(values.version).toBeUndefined()
    expect(values.run_command).toBeUndefined()
    expect(values.endpoint_url).toBeUndefined()
  })
})

describe('renderUsageMarkdown', () => {
  it.each([
    ['known placeholder', 'v{{version}}', 'v1.0.0'],
    ['whitespace inside braces', 'v{{ version }}', 'v1.0.0'],
    ['unknown placeholder kept', '{{nope}}', '{{nope}}'],
    ['unset placeholder kept', '{{endpoint_url}}', '{{endpoint_url}}'],
  ])('%s', (_, input, want) => {
    expect(renderUsageMarkdown(input, { version: '1.0.0' })).toBe(want)
  })
})

describe('starterUsageMarkdown', () => {
  it('only mentions what the server provides', () => {
    const md = starterUsageMarkdown({ run_command: 'npx -y x' })
    expect(md).toContain('{{run_command}}')
    expect(md).not.toContain('{{endpoint_url}}')
  })
})

describe('applyMarkdownFormat', () => {
  it.each([
    ['bold wraps the selection', 'a word', 2, 6, 'bold', 'a **word**', [4, 8]],
    ['bold without selection inserts a placeholder', 'ab', 1, 1, 'bold', 'a**bold text**b', [3, 12]],
    ['heading prefixes the current line', 'one\ntwo', 5, 5, 'heading', 'one\n## two', [8, 8]],
    ['list prefixes every selected line', 'a\nb', 0, 3, 'list', '- a\n- b', [2, 7]],
    ['link wraps as link text', 'docs', 0, 4, 'link', '[docs](https://)', [1, 5]],
  ] as const)('%s', (_, value, start, end, format, want, [selStart, selEnd]) => {
    const edit = applyMarkdownFormat(value, start, end, format)
    expect(edit.value).toBe(want)
    expect([edit.selectionStart, edit.selectionEnd]).toEqual([selStart, selEnd])
  })

  it('insertText replaces the selection and puts the caret after it', () => {
    expect(insertText('a XX b', 2, 4, '{{version}}')).toEqual({
      value: 'a {{version}} b',
      selectionStart: 13,
      selectionEnd: 13,
    })
  })
})
