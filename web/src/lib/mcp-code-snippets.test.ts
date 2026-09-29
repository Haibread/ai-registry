import { describe, it, expect } from 'vitest'
import {
  SNIPPET_LANGUAGES,
  exampleToolCall,
  generateCodeSnippet,
  toSnippetConnection,
  type SnippetConnection,
  type SnippetLanguage,
} from './mcp-code-snippets'

const http: SnippetConnection = { kind: 'streamable_http', url: 'https://mcp.example.com/mcp' }
const sse: SnippetConnection = { kind: 'sse', url: 'https://mcp.example.com/sse' }
const stdio: SnippetConnection = { kind: 'stdio', command: 'npx', args: ['-y', '@acme/server@1.0.0'] }
const call = { name: 'search_docs', args: [['query', '...'] as [string, string]] }

describe('toSnippetConnection', () => {
  it.each([
    [{ serverName: 's', transport: 'stdio' as const, command: 'uvx', args: ['x==1'] }, { kind: 'stdio', command: 'uvx', args: ['x==1'] }],
    [{ serverName: 's', transport: 'sse' as const, url: 'https://h/sse' }, { kind: 'sse', url: 'https://h/sse' }],
    [{ serverName: 's', transport: 'streamable_http' as const, url: 'https://h/mcp' }, { kind: 'streamable_http', url: 'https://h/mcp' }],
  ])('maps %o', (params, expected) => {
    expect(toSnippetConnection(params)).toEqual(expected)
  })
})

describe('exampleToolCall', () => {
  it('returns null without tools', () => {
    expect(exampleToolCall(undefined)).toBeNull()
    expect(exampleToolCall([])).toBeNull()
  })

  it('uses the first tool and only its required arguments', () => {
    const result = exampleToolCall([
      {
        name: 'search_docs',
        input_schema: {
          type: 'object',
          properties: {
            query: { type: 'string' },
            limit: { type: 'integer' },
            lang: { type: 'string' },
          },
          required: ['query', 'limit'],
        },
      },
      { name: 'other' },
    ])
    expect(result).toEqual({ name: 'search_docs', args: [['query', '...'], ['limit', 1]] })
  })

  it.each([
    [{ type: 'string', default: 'fr' }, 'fr'],
    [{ type: 'string', enum: ['asc', 'desc'] }, 'asc'],
    [{ type: 'number' }, 1],
    [{ type: 'boolean' }, true],
    [{ type: 'array' }, []],
    [{ type: 'object' }, {}],
    [{ type: ['null', 'integer'] }, 1],
    [{}, '...'],
  ])('derives an example value from %o', (prop, expected) => {
    const result = exampleToolCall([
      { name: 't', input_schema: { properties: { a: prop }, required: ['a'] } },
    ])
    expect(result?.args).toEqual([['a', expected]])
  })

  it('tolerates a tool without input schema', () => {
    expect(exampleToolCall([{ name: 'ping' }])).toEqual({ name: 'ping', args: [] })
  })
})

describe('generateCodeSnippet', () => {
  const languages = SNIPPET_LANGUAGES.map((l) => l.value)

  it.each(languages)('%s pins the SDK version in the install line', (lang) => {
    const pinned: Record<SnippetLanguage, RegExp> = {
      python: /mcp==2\.2\.0/,
      typescript: /@modelcontextprotocol\/sdk@1\.30\.1/,
      go: /go-sdk@v1\.8\.0/,
      java: /<version>2\.0\.1<\/version>/,
      rust: /rmcp = \{ version = "3\.5\.0"/,
    }
    expect(generateCodeSnippet(lang, http, call).install).toMatch(pinned[lang])
  })

  it.each(languages)('%s calls the example tool when one is known', (lang) => {
    const { code } = generateCodeSnippet(lang, http, call)
    expect(code).toContain('"search_docs"')
    expect(code).toContain('"...')
  })

  it.each(languages)('%s stops at listing tools when none is known', (lang) => {
    const { code } = generateCodeSnippet(lang, http, null)
    expect(code).not.toMatch(/call_?[tT]ool|CallTool/)
  })

  it.each([
    ['python', http, 'Client("https://mcp.example.com/mcp")'],
    ['python', sse, 'sse_client("https://mcp.example.com/sse")'],
    ['python', stdio, 'StdioServerParameters(command="npx", args=["-y","@acme/server@1.0.0"])'],
    ['typescript', http, 'new StreamableHTTPClientTransport(new URL("https://mcp.example.com/mcp"))'],
    ['typescript', sse, 'new SSEClientTransport(new URL("https://mcp.example.com/sse"))'],
    ['typescript', stdio, 'new StdioClientTransport({ command: "npx", args: ["-y","@acme/server@1.0.0"] })'],
    ['go', http, '&mcp.StreamableClientTransport{Endpoint: "https://mcp.example.com/mcp"}'],
    ['go', sse, '&mcp.SSEClientTransport{Endpoint: "https://mcp.example.com/sse"}'],
    ['go', stdio, 'exec.Command("npx", "-y", "@acme/server@1.0.0")'],
    ['java', http, 'HttpClientStreamableHttpTransport.builder("https://mcp.example.com")\n            .endpoint("/mcp")'],
    ['java', sse, 'HttpClientSseClientTransport.builder("https://mcp.example.com")\n            .sseEndpoint("/sse")'],
    ['java', stdio, 'ServerParameters.builder("npx").args("-y", "@acme/server@1.0.0")'],
    ['rust', http, 'StreamableHttpClientTransport::from_uri("https://mcp.example.com/mcp")'],
    ['rust', stdio, 'cmd.arg("-y").arg("@acme/server@1.0.0")'],
  ] as const)('%s builds the transport for %o', (lang, conn, expected) => {
    expect(generateCodeSnippet(lang, conn, call).code).toContain(expected)
  })

  it('has no Rust snippet for SSE', () => {
    expect(generateCodeSnippet('rust', sse, call).code).toBeNull()
  })

  it('enables TLS for the Rust HTTP transport and process spawning for stdio', () => {
    expect(generateCodeSnippet('rust', http, call).install).toContain('"reqwest"')
    expect(generateCodeSnippet('rust', stdio, call).install).toContain('"process"')
  })

  it('imports os/exec in Go only for stdio', () => {
    expect(generateCodeSnippet('go', stdio, call).code).toContain('"os/exec"')
    expect(generateCodeSnippet('go', http, call).code).not.toContain('"os/exec"')
  })

  it.each([
    ['python', '{ "q": "...", "n": 1, "on": True, "tags": [], "opts": {} }'],
    ['typescript', '{ q: "...", n: 1, on: true, tags: [], opts: {} }'],
    ['go', 'map[string]any{"q": "...", "n": 1, "on": true, "tags": []any{}, "opts": map[string]any{}}'],
    ['java', 'Map.of("q", "...", "n", 1, "on", true, "tags", List.of(), "opts", Map.of())'],
    ['rust', 'json!({ "q": "...", "n": 1, "on": true, "tags": [], "opts": {} })'],
  ] as const)('%s renders typed argument literals', (lang, expected) => {
    const typed = {
      name: 't',
      args: [['q', '...'], ['n', 1], ['on', true], ['tags', []], ['opts', {}]] as [string, never][],
    }
    expect(generateCodeSnippet(lang, http, typed).code).toContain(expected)
  })

  it('quotes TypeScript keys that are not identifiers', () => {
    const { code } = generateCodeSnippet('typescript', http, { name: 't', args: [['max-results', 1]] })
    expect(code).toContain('{ "max-results": 1 }')
  })

  it('switches to Map.ofEntries in Java past ten arguments', () => {
    const args = Array.from({ length: 11 }, (_, i) => [`a${i}`, 1] as [string, number])
    expect(generateCodeSnippet('java', http, { name: 't', args }).code).toContain('Map.ofEntries(Map.entry("a0", 1)')
  })
})
