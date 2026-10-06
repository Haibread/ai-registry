import type { components } from '@/lib/schema'
import { getInstallCommand, isRemoteTransport } from '@/lib/utils'

type MCPServer = components['schemas']['MCPServer']
export type UsageVersion = Pick<NonNullable<MCPServer['latest_version']>, 'version' | 'packages' | 'remotes' | 'tools'>

export const MAX_USAGE_MARKDOWN_LENGTH = 20000

export interface UsageVariable {
  name: string
  description: string
}

export const USAGE_VARIABLES: UsageVariable[] = [
  { name: 'name', description: 'Server name' },
  { name: 'namespace', description: 'Publisher namespace' },
  { name: 'slug', description: 'Server slug' },
  { name: 'version', description: 'Latest published version' },
  { name: 'run_command', description: 'Run command of the first local package' },
  { name: 'endpoint_url', description: 'URL of the first remote endpoint' },
]

export type UsageValues = Partial<Record<string, string>>

/** Values for the placeholders, from the server and the version it shows. */
export function usageValues(server: MCPServer, lv: UsageVersion | undefined = server.latest_version): UsageValues {
  const packages = lv?.packages ?? []
  const local = packages.find((p) => !isRemoteTransport(p.transport.type))
  const remotePackage = packages.find((p) => isRemoteTransport(p.transport.type) && p.transport.url)
  return {
    name: server.name,
    namespace: server.namespace,
    slug: server.slug,
    version: lv?.version,
    run_command: local && getInstallCommand(local),
    endpoint_url: lv?.remotes?.[0]?.url ?? remotePackage?.transport.url,
  }
}

/** Substitutes `{{name}}` placeholders; unknown or unset ones stay as written. */
export function renderUsageMarkdown(markdown: string, values: UsageValues): string {
  return markdown.replace(/\{\{\s*([a-z_]+)\s*\}\}/g, (match, name: string) => values[name] ?? match)
}

/** A starting point equivalent to the generated connection instructions. */
export function starterUsageMarkdown(values: UsageValues): string {
  const parts = ['## Connect']
  if (values.run_command) {
    parts.push('Run the server locally:', '```sh\n{{run_command}}\n```')
  }
  if (values.endpoint_url) {
    parts.push('Connect to the hosted endpoint: `{{endpoint_url}}`')
  }
  return parts.join('\n\n') + '\n'
}

export type MarkdownFormat = 'heading' | 'bold' | 'italic' | 'code' | 'link' | 'list' | 'codeblock'

export interface TextEdit {
  value: string
  selectionStart: number
  selectionEnd: number
}

/** Inserts `text` over the selection and places the caret after it. */
export function insertText(value: string, start: number, end: number, text: string): TextEdit {
  const caret = start + text.length
  return { value: value.slice(0, start) + text + value.slice(end), selectionStart: caret, selectionEnd: caret }
}

function wrap(value: string, start: number, end: number, before: string, after: string, placeholder: string): TextEdit {
  const inner = value.slice(start, end) || placeholder
  return {
    value: value.slice(0, start) + before + inner + after + value.slice(end),
    selectionStart: start + before.length,
    selectionEnd: start + before.length + inner.length,
  }
}

function prefixLines(value: string, start: number, end: number, prefix: string): TextEdit {
  const lineStart = value.lastIndexOf('\n', start - 1) + 1
  const block = value.slice(lineStart, end)
  const prefixed = block.split('\n').map((line) => prefix + line).join('\n')
  return {
    value: value.slice(0, lineStart) + prefixed + value.slice(end),
    selectionStart: start + prefix.length,
    selectionEnd: lineStart + prefixed.length,
  }
}

/** Applies a toolbar format to the selection of a Markdown textarea. */
export function applyMarkdownFormat(value: string, start: number, end: number, format: MarkdownFormat): TextEdit {
  switch (format) {
    case 'heading':
      return prefixLines(value, start, end, '## ')
    case 'list':
      return prefixLines(value, start, end, '- ')
    case 'bold':
      return wrap(value, start, end, '**', '**', 'bold text')
    case 'italic':
      return wrap(value, start, end, '_', '_', 'italic text')
    case 'code':
      return wrap(value, start, end, '`', '`', 'code')
    case 'link':
      return wrap(value, start, end, '[', '](https://)', 'link text')
    case 'codeblock':
      return wrap(value, start, end, '```sh\n', '\n```', 'command')
  }
}
