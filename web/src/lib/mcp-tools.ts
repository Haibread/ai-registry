/**
 * Reads the publisher-declared `tools[]` of an MCP server version: parameters
 * from the input JSON Schema, behaviour from the MCP tool annotations, and an
 * example call. Shared by the Tools tab and the client code snippets.
 */

import type { components } from '@/lib/schema'

export type MCPTool = components['schemas']['MCPTool']

export type ArgValue = string | number | boolean | [] | Record<string, never>

export interface ToolCall {
  name: string
  args: [string, ArgValue][]
}

export interface ToolParameter {
  name: string
  type: string | null
  required: boolean
  description: string | null
  enumValues: unknown[] | null
  defaultValue: { value: unknown } | null
}

export type BehaviorKey = 'readOnly' | 'destructive' | 'idempotent' | 'openWorld'

/** `unset` falls back to the spec default; `not-applicable` is a hint the spec
 *  only reads when the tool is not read-only. */
export type BehaviorState = 'yes' | 'no' | 'unset' | 'not-applicable'

export interface ToolBehavior {
  key: BehaviorKey
  state: BehaviorState
}

interface JsonSchemaProperty {
  type?: unknown
  format?: unknown
  items?: unknown
  description?: unknown
  enum?: unknown
  default?: unknown
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function isArgValue(v: unknown): v is string | number | boolean {
  return typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean'
}

function schemaParts(tool: MCPTool) {
  const schema = tool.input_schema ?? {}
  return {
    schema,
    properties: isRecord(schema.properties) ? schema.properties : {},
    required: Array.isArray(schema.required)
      ? schema.required.filter((r): r is string => typeof r === 'string')
      : [],
  }
}

function exampleValue(prop: JsonSchemaProperty): ArgValue {
  if (isArgValue(prop.default)) return prop.default
  if (Array.isArray(prop.enum) && isArgValue(prop.enum[0])) return prop.enum[0]
  const type = Array.isArray(prop.type) ? prop.type.find((t) => t !== 'null') : prop.type
  switch (type) {
    case 'integer':
    case 'number':
      return 1
    case 'boolean':
      return true
    case 'array':
      return []
    case 'object':
      return {}
    default:
      return '...'
  }
}

/** A call to `tool` with an example value for each required argument. */
export function toolCall(tool: MCPTool): ToolCall {
  const { properties, required } = schemaParts(tool)
  return {
    name: tool.name,
    args: required.map((key) => {
      const prop = properties[key]
      return [key, exampleValue(isRecord(prop) ? prop : {})]
    }),
  }
}

/** The JSON-RPC request an MCP client sends for `toolCall(tool)`. */
export function jsonRpcToolCall(tool: MCPTool): string {
  const call = toolCall(tool)
  return JSON.stringify(
    {
      jsonrpc: '2.0',
      id: 1,
      method: 'tools/call',
      params: { name: call.name, arguments: Object.fromEntries(call.args) },
    },
    null,
    2,
  )
}

function typeLabel(prop: JsonSchemaProperty): string | null {
  const types = (Array.isArray(prop.type) ? prop.type : [prop.type]).filter(
    (t): t is string => typeof t === 'string',
  )
  if (types.length === 0) return null
  return types
    .map((t) => {
      if (t === 'array' && isRecord(prop.items) && typeof prop.items.type === 'string') {
        return `${prop.items.type}[]`
      }
      return typeof prop.format === 'string' && t !== 'null' ? `${t} (${prop.format})` : t
    })
    .join(' | ')
}

/**
 * The top-level properties of the tool's input schema, required ones first.
 * Null when the schema has no `properties` but composes others (`oneOf`,
 * `$ref`, …): flattening that into a table would misstate it.
 */
export function toolParameters(tool: MCPTool): ToolParameter[] | null {
  const { schema, properties, required } = schemaParts(tool)
  if (!isRecord(schema.properties) && ['oneOf', 'anyOf', 'allOf', '$ref'].some((k) => k in schema)) {
    return null
  }
  return Object.entries(properties)
    .map(([name, raw]): ToolParameter => {
      const prop: JsonSchemaProperty = isRecord(raw) ? raw : {}
      return {
        name,
        type: typeLabel(prop),
        required: required.includes(name),
        description: typeof prop.description === 'string' ? prop.description : null,
        enumValues: Array.isArray(prop.enum) ? prop.enum : null,
        defaultValue: 'default' in prop ? { value: prop.default } : null,
      }
    })
    .sort((a, b) => Number(b.required) - Number(a.required))
}

function hint(tool: MCPTool, key: string): boolean | undefined {
  const v = tool.annotations?.[key]
  return typeof v === 'boolean' ? v : undefined
}

/** The four MCP behaviour hints, in display order. */
export function toolBehaviors(tool: MCPTool): ToolBehavior[] {
  const readOnly = hint(tool, 'readOnlyHint')
  const state = (v: boolean | undefined): BehaviorState =>
    v === undefined ? 'unset' : v ? 'yes' : 'no'
  const unlessReadOnly = (v: boolean | undefined): BehaviorState =>
    readOnly ? 'not-applicable' : state(v)
  return [
    { key: 'readOnly', state: state(readOnly) },
    { key: 'destructive', state: unlessReadOnly(hint(tool, 'destructiveHint')) },
    { key: 'idempotent', state: unlessReadOnly(hint(tool, 'idempotentHint')) },
    { key: 'openWorld', state: state(hint(tool, 'openWorldHint')) },
  ]
}

export function toolTitle(tool: MCPTool): string | null {
  const title = tool.annotations?.title
  return typeof title === 'string' && title !== tool.name ? title : null
}

const KNOWN_ANNOTATIONS = new Set(['title', 'readOnlyHint', 'destructiveHint', 'idempotentHint', 'openWorldHint'])

/** Annotations outside the MCP spec, as published. */
export function otherAnnotations(tool: MCPTool): [string, unknown][] {
  return Object.entries(tool.annotations ?? {}).filter(([k]) => !KNOWN_ANNOTATIONS.has(k))
}
