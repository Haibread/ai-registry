// @vitest-environment node

import { describe, it, expect } from 'vitest'
import { jsonRpcToolCall, otherAnnotations, toolBehaviors, toolParameters, toolTitle, type MCPTool } from './mcp-tools'

const state = (tool: MCPTool) => Object.fromEntries(toolBehaviors(tool).map((b) => [b.key, b.state]))

describe('toolParameters', () => {
  it('reads the properties, required ones first', () => {
    const params = toolParameters({
      name: 't',
      input_schema: {
        type: 'object',
        properties: {
          limit: { type: 'integer', default: 0 },
          state: { type: 'string', enum: ['open', 'closed'], description: 'Issue state' },
          labels: { type: 'array', items: { type: 'string' } },
          url: { type: ['string', 'null'], format: 'uri' },
          blob: {},
        },
        required: ['state'],
      },
    })
    expect(params?.map((p) => p.name)).toEqual(['state', 'limit', 'labels', 'url', 'blob'])
    expect(params?.[0]).toEqual({
      name: 'state',
      type: 'string',
      required: true,
      description: 'Issue state',
      enumValues: ['open', 'closed'],
      defaultValue: null,
    })
    expect(params?.find((p) => p.name === 'limit')?.defaultValue).toEqual({ value: 0 })
    expect(params?.find((p) => p.name === 'labels')?.type).toBe('string[]')
    expect(params?.find((p) => p.name === 'url')?.type).toBe('string (uri) | null')
    expect(params?.find((p) => p.name === 'blob')?.type).toBeNull()
  })

  it('is empty without an input schema', () => {
    expect(toolParameters({ name: 't' })).toEqual([])
  })

  it('refuses to flatten a composed schema', () => {
    expect(toolParameters({ name: 't', input_schema: { oneOf: [{ type: 'object' }] } })).toBeNull()
  })
})

describe('toolBehaviors', () => {
  it('reports declared hints and leaves undeclared ones unset', () => {
    expect(state({ name: 't', annotations: { destructiveHint: true, openWorldHint: false } })).toEqual({
      readOnly: 'unset',
      destructive: 'yes',
      idempotent: 'unset',
      openWorld: 'no',
    })
  })

  it('marks the write hints not applicable on a read-only tool', () => {
    expect(state({ name: 't', annotations: { readOnlyHint: true, destructiveHint: true } })).toEqual({
      readOnly: 'yes',
      destructive: 'not-applicable',
      idempotent: 'not-applicable',
      openWorld: 'unset',
    })
  })

  it('ignores hints that are not booleans', () => {
    expect(state({ name: 't', annotations: { readOnlyHint: 'yes' } }).readOnly).toBe('unset')
  })
})

describe('toolTitle', () => {
  it('returns the annotation title unless it repeats the name', () => {
    expect(toolTitle({ name: 'merge', annotations: { title: 'Merge pull request' } })).toBe('Merge pull request')
    expect(toolTitle({ name: 'merge', annotations: { title: 'merge' } })).toBeNull()
    expect(toolTitle({ name: 'merge' })).toBeNull()
  })
})

describe('otherAnnotations', () => {
  it('keeps only the keys outside the MCP spec', () => {
    expect(otherAnnotations({ name: 't', annotations: { title: 'T', readOnlyHint: true, audience: 'ops' } }))
      .toEqual([['audience', 'ops']])
  })
})

describe('jsonRpcToolCall', () => {
  it('builds a tools/call request with the required arguments', () => {
    const body = JSON.parse(jsonRpcToolCall({
      name: 'issue_close',
      input_schema: {
        properties: { number: { type: 'integer' }, reason: { enum: ['completed'] }, note: { type: 'string' } },
        required: ['number', 'reason'],
      },
    }))
    expect(body).toEqual({
      jsonrpc: '2.0',
      id: 1,
      method: 'tools/call',
      params: { name: 'issue_close', arguments: { number: 1, reason: 'completed' } },
    })
  })
})
