import { describe, it, expect } from 'vitest'
import { MCP_HOSTS, packageToConfigParams } from './mcp-host-configs'

describe('MCP host configs', () => {
  it('defines at least 4 hosts', () => {
    expect(MCP_HOSTS.length).toBeGreaterThanOrEqual(4)
  })

  it('each host has name, configPath, and generate', () => {
    for (const host of MCP_HOSTS) {
      expect(host.name).toBeTruthy()
      expect(host.configPath).toBeTruthy()
      expect(typeof host.generate).toBe('function')
    }
  })
})

describe('packageToConfigParams', () => {
  it('generates stdio params for npm packages', () => {
    const result = packageToConfigParams('my-server', {
      registryType: 'npm',
      identifier: '@acme/mcp-server',
      version: '1.0.0',
      transport: { type: 'stdio' },
    })
    expect(result.transport).toBe('stdio')
    expect(result.command).toBe('npx')
    expect(result.args).toEqual(['-y', '@acme/mcp-server@1.0.0'])
  })

  it('generates stdio params for pypi packages', () => {
    const result = packageToConfigParams('my-server', {
      registryType: 'pypi',
      identifier: 'my-mcp',
      version: '2.0.0',
      transport: { type: 'stdio' },
    })
    expect(result.command).toBe('uvx')
    expect(result.args).toEqual(['my-mcp==2.0.0'])
  })

  it('generates stdio params for docker packages', () => {
    const result = packageToConfigParams('my-server', {
      registryType: 'docker',
      identifier: 'ghcr.io/acme/mcp',
      version: 'latest',
      transport: { type: 'stdio' },
    })
    expect(result.command).toBe('docker')
    expect(result.args).toContain('ghcr.io/acme/mcp:latest')
  })

  it('generates remote params for SSE transport', () => {
    const result = packageToConfigParams('my-server', {
      registryType: 'npm',
      identifier: '@acme/server',
      version: '1.0.0',
      transport: { type: 'sse', url: 'https://example.com/sse' },
    })
    expect(result.transport).toBe('sse')
    expect(result.url).toBe('https://example.com/sse')
  })

  it('generates remote params for streamable_http transport', () => {
    const result = packageToConfigParams('srv', {
      registryType: 'npm',
      identifier: '@acme/server',
      version: '1.0.0',
      transport: { type: 'streamable_http', url: 'https://example.com/mcp' },
    })
    expect(result.transport).toBe('streamable_http')
    expect(result.url).toBe('https://example.com/mcp')
  })
})

describe('host generate', () => {
  const stdioParams = packageToConfigParams('test-server', {
    registryType: 'npm',
    identifier: '@acme/test',
    version: '1.0.0',
    transport: { type: 'stdio' },
  })

  it('Claude Desktop generates valid JSON with mcpServers key', () => {
    const host = MCP_HOSTS.find((h) => h.name === 'Claude Desktop')!
    const result = JSON.parse(host.generate(stdioParams))
    expect(result.mcpServers['test-server']).toBeDefined()
    expect(result.mcpServers['test-server'].command).toBe('npx')
  })

  it('VS Code generates servers key (not mcpServers)', () => {
    const host = MCP_HOSTS.find((h) => h.name === 'VS Code')!
    const result = JSON.parse(host.generate(stdioParams))
    expect(result.servers['test-server']).toBeDefined()
  })

  it('remote transport includes url in output', () => {
    const remoteParams = packageToConfigParams('srv', {
      registryType: 'npm',
      identifier: '@acme/server',
      version: '1.0.0',
      transport: { type: 'sse', url: 'https://example.com/sse' },
    })
    const host = MCP_HOSTS[0]
    const result = JSON.parse(host.generate(remoteParams))
    expect(result.mcpServers['srv'].url).toBe('https://example.com/sse')
  })
})
