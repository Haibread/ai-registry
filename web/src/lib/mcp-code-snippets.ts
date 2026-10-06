/**
 * Client code snippets for connecting to an MCP server with the official SDKs.
 *
 * The SDK versions are pinned: the snippets were checked against these exact
 * releases, and several SDKs broke their client API between majors.
 */

import type { MCPConfigParams } from '@/lib/mcp-host-configs'
import { toolCall, type ArgValue, type MCPTool, type ToolCall } from '@/lib/mcp-tools'

export type SnippetLanguage = 'python' | 'typescript' | 'go' | 'java' | 'rust'

export const SNIPPET_LANGUAGES: { value: SnippetLanguage; label: string }[] = [
  { value: 'python', label: 'Python' },
  { value: 'typescript', label: 'TypeScript' },
  { value: 'go', label: 'Go' },
  { value: 'java', label: 'Java' },
  { value: 'rust', label: 'Rust' },
]

export type SnippetConnection =
  | { kind: 'stdio'; command: string; args: string[] }
  | { kind: 'streamable_http' | 'sse'; url: string }

export interface CodeSnippet {
  install: string
  /** Null when the SDK has no client for the connection's transport. */
  code: string | null
}

export function toSnippetConnection(params: MCPConfigParams): SnippetConnection {
  if (params.transport === 'stdio') {
    return { kind: 'stdio', command: params.command ?? '', args: params.args ?? [] }
  }
  return { kind: params.transport === 'sse' ? 'sse' : 'streamable_http', url: params.url ?? '' }
}

/**
 * The call a snippet demonstrates: the version's first declared tool, with
 * an example value for each required argument. Null when no tool is known,
 * so the snippet never invents a tool name.
 */
export function exampleToolCall(tools: MCPTool[] | undefined): ToolCall | null {
  const tool = tools?.[0]
  return tool ? toolCall(tool) : null
}

const str = (s: string) => JSON.stringify(s)

function literal(lang: SnippetLanguage, v: ArgValue): string {
  if (Array.isArray(v)) {
    return { python: '[]', typescript: '[]', go: '[]any{}', java: 'List.of()', rust: '[]' }[lang]
  }
  if (typeof v === 'object') {
    return { python: '{}', typescript: '{}', go: 'map[string]any{}', java: 'Map.of()', rust: '{}' }[lang]
  }
  if (typeof v === 'boolean' && lang === 'python') return v ? 'True' : 'False'
  return typeof v === 'string' ? str(v) : String(v)
}

function argsLiteral(lang: SnippetLanguage, args: ToolCall['args']): string {
  const entries = args.map(([k, v]) => [k, literal(lang, v)] as const)
  switch (lang) {
    case 'python':
    case 'rust':
      return `{${entries.map(([k, v]) => ` ${str(k)}: ${v}`).join(',')}${entries.length ? ' ' : ''}}`
    case 'typescript':
      return `{${entries
        .map(([k, v]) => ` ${/^[A-Za-z_$][\w$]*$/.test(k) ? k : str(k)}: ${v}`)
        .join(',')}${entries.length ? ' ' : ''}}`
    case 'go':
      return `map[string]any{${entries.map(([k, v]) => `${str(k)}: ${v}`).join(', ')}}`
    case 'java':
      // Map.of stops at ten pairs.
      return entries.length <= 10
        ? `Map.of(${entries.map(([k, v]) => `${str(k)}, ${v}`).join(', ')})`
        : `Map.ofEntries(${entries.map(([k, v]) => `Map.entry(${str(k)}, ${v})`).join(', ')})`
  }
}

function install(lang: SnippetLanguage, conn: SnippetConnection): string {
  switch (lang) {
    case 'python':
      return 'pip install "mcp==2.2.0"'
    case 'typescript':
      return 'npm install @modelcontextprotocol/sdk@1.30.1'
    case 'go':
      return 'go get github.com/modelcontextprotocol/go-sdk@v1.8.0'
    case 'java':
      return `<dependency>
  <groupId>io.modelcontextprotocol.sdk</groupId>
  <artifactId>mcp</artifactId>
  <version>2.0.1</version>
</dependency>`
    case 'rust': {
      const stdio = conn.kind === 'stdio'
      // Without "reqwest" the HTTP transport is built with no TLS backend and
      // every https:// URL fails to connect.
      const features = stdio
        ? ['client', 'transport-child-process']
        : ['client', 'reqwest', 'transport-streamable-http-client-reqwest']
      return `[dependencies]
rmcp = { version = "3.5.0", default-features = false, features = [${features.map(str).join(', ')}] }
tokio = { version = "1", features = ["macros", "rt-multi-thread"${stdio ? ', "process"' : ''}] }
serde_json = "1"
anyhow = "1"`
    }
  }
}

function python(conn: SnippetConnection, call: ToolCall | null): string {
  const imports =
    conn.kind === 'stdio'
      ? 'from mcp import Client, StdioServerParameters'
      : conn.kind === 'sse'
        ? 'from mcp import Client\nfrom mcp.client.sse import sse_client'
        : 'from mcp import Client'
  const target =
    conn.kind === 'stdio'
      ? `StdioServerParameters(command=${str(conn.command)}, args=${JSON.stringify(conn.args)})`
      : conn.kind === 'sse'
        ? `sse_client(${str(conn.url)})`
        : str(conn.url)
  const callLines = call
    ? `
        result = await client.call_tool(${str(call.name)}, ${argsLiteral('python', call.args)})
        print(result.content)`
    : ''
  return `import asyncio

${imports}


async def main() -> None:
    async with Client(${target}) as client:
        tools = await client.list_tools()
        for tool in tools.tools:
            print(tool.name)${callLines}


asyncio.run(main())
`
}

function typescript(conn: SnippetConnection, call: ToolCall | null): string {
  const [imp, transport] =
    conn.kind === 'stdio'
      ? [
          `import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";`,
          `new StdioClientTransport({ command: ${str(conn.command)}, args: ${JSON.stringify(conn.args)} })`,
        ]
      : conn.kind === 'sse'
        ? [
            `import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js";`,
            `new SSEClientTransport(new URL(${str(conn.url)}))`,
          ]
        : [
            `import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";`,
            `new StreamableHTTPClientTransport(new URL(${str(conn.url)}))`,
          ]
  const callLines = call
    ? `
const result = await client.callTool({ name: ${str(call.name)}, arguments: ${argsLiteral('typescript', call.args)} });
console.log(JSON.stringify(result.content));
`
    : ''
  return `import { Client } from "@modelcontextprotocol/sdk/client/index.js";
${imp}

const client = new Client({ name: "my-app", version: "1.0.0" });
await client.connect(${transport});

const { tools } = await client.listTools();
for (const tool of tools) console.log(tool.name);
${callLines}
await client.close();
`
}

function go(conn: SnippetConnection, call: ToolCall | null): string {
  const transport =
    conn.kind === 'stdio'
      ? `&mcp.CommandTransport{Command: exec.Command(${[conn.command, ...conn.args].map(str).join(', ')})}`
      : conn.kind === 'sse'
        ? `&mcp.SSEClientTransport{Endpoint: ${str(conn.url)}}`
        : `&mcp.StreamableClientTransport{Endpoint: ${str(conn.url)}}`
  const callLines = call
    ? `

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      ${str(call.name)},
		Arguments: ${argsLiteral('go', call.args)},
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			fmt.Println(t.Text)
		}
	}`
    : ''
  return `package main

import (
	"context"
	"fmt"
	"log"
${conn.kind === 'stdio' ? '\t"os/exec"\n' : ''}
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "my-app", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, ${transport}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(tool.Name)
	}${callLines}
}
`
}

// The Java HTTP transports take the server origin and the endpoint path apart.
function splitUrl(url: string): [string, string] {
  try {
    const u = new URL(url)
    return [u.origin, `${u.pathname}${u.search}`]
  } catch {
    return [url, '']
  }
}

function java(conn: SnippetConnection, call: ToolCall | null): string {
  let imports: string[]
  let transport: string
  if (conn.kind === 'stdio') {
    imports = [
      'io.modelcontextprotocol.client.transport.ServerParameters',
      'io.modelcontextprotocol.client.transport.StdioClientTransport',
      'io.modelcontextprotocol.json.McpJsonDefaults',
    ]
    const args = conn.args.length ? `.args(${conn.args.map(str).join(', ')})` : ''
    transport = `var params = ServerParameters.builder(${str(conn.command)})${args}.build();
        var transport = new StdioClientTransport(params, McpJsonDefaults.getMapper());`
  } else {
    const [base, path] = splitUrl(conn.url)
    const sse = conn.kind === 'sse'
    const cls = sse ? 'HttpClientSseClientTransport' : 'HttpClientStreamableHttpTransport'
    imports = [`io.modelcontextprotocol.client.transport.${cls}`]
    transport = `var transport = ${cls}.builder(${str(base)})${
      path ? `\n            .${sse ? 'sseEndpoint' : 'endpoint'}(${str(path)})` : ''
    }
            .build();`
  }
  const needsList = call?.args.some(([, v]) => Array.isArray(v)) ?? false
  const javaImports = [...(needsList ? ['java.util.List'] : []), ...(call ? ['java.util.Map'] : [])]
  const callLines = call
    ? `
            McpSchema.CallToolResult result = client.callTool(McpSchema.CallToolRequest.builder(${str(call.name)})
                .arguments(${argsLiteral('java', call.args)})
                .build());
            for (McpSchema.Content content : result.content()) {
                if (content instanceof McpSchema.TextContent text) {
                    System.out.println(text.text());
                }
            }`
    : ''
  return `${javaImports.map((i) => `import ${i};\n`).join('')}${javaImports.length ? '\n' : ''}import io.modelcontextprotocol.client.McpClient;
import io.modelcontextprotocol.client.McpSyncClient;
${imports.map((i) => `import ${i};`).join('\n')}
import io.modelcontextprotocol.spec.McpSchema;

public class Main {
    public static void main(String[] args) {
        ${transport}
        try (McpSyncClient client = McpClient.sync(transport)
                .clientInfo(McpSchema.Implementation.builder("my-app", "1.0.0").build())
                .build()) {
            client.initialize();
            for (McpSchema.Tool tool : client.listTools().tools()) {
                System.out.println(tool.name());
            }${callLines}
        }
    }
}
`
}

function rust(conn: SnippetConnection, call: ToolCall | null): string | null {
  if (conn.kind === 'sse') return null
  const stdio = conn.kind === 'stdio'
  const withArgs = stdio && conn.args.length > 0
  const transportUse = !stdio
    ? 'StreamableHttpClientTransport'
    : withArgs
      ? '{ConfigureCommandExt, TokioChildProcess}'
      : 'TokioChildProcess'
  const uses = call
    ? `use rmcp::{ServiceExt, model::CallToolRequestParams, transport::${transportUse}};
use serde_json::json;`
    : `use rmcp::{ServiceExt, transport::${transportUse}};`
  const transport = !stdio
    ? `let transport = StreamableHttpClientTransport::from_uri(${str(conn.url)});`
    : withArgs
      ? `let transport = TokioChildProcess::new(Command::new(${str(conn.command)}).configure(|cmd| {
        cmd${conn.args.map((a) => `.arg(${str(a)})`).join('')};
    }))?;`
      : `let transport = TokioChildProcess::new(Command::new(${str(conn.command)}))?;`
  const callLines = call
    ? `

    let args = json!(${argsLiteral('rust', call.args)}).as_object().cloned().unwrap();
    let result = client
        .call_tool(CallToolRequestParams::new(${str(call.name)}).with_arguments(args))
        .await?;
    println!("{}", serde_json::to_string(&result.content)?);`
    : ''
  return `${uses}
${stdio ? 'use tokio::process::Command;\n' : ''}
#[tokio::main]
async fn main() -> anyhow::Result<()> {
    ${transport}
    let client = ().serve(transport).await?;

    for tool in client.list_all_tools().await? {
        println!("{}", tool.name);
    }${callLines}

    client.cancel().await?;
    Ok(())
}
`
}

export function generateCodeSnippet(
  lang: SnippetLanguage,
  conn: SnippetConnection,
  call: ToolCall | null,
): CodeSnippet {
  const generators = { python, typescript, go, java, rust }
  return { install: install(lang, conn), code: generators[lang](conn, call) }
}
