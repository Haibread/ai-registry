/**
 * MCP host configuration templates.
 *
 * Each host describes:
 * - Where its config file lives
 * - How to format the mcpServers JSON block
 * - Whether it supports stdio / remote transports
 */

export interface MCPHostConfig {
  /** Display name */
  name: string
  /** Typical config file path (shown to user) */
  configPath: string
  /** Generate the JSON config snippet for a given server */
  generate: (params: MCPConfigParams) => string
}

export interface MCPConfigParams {
  /** Server name (used as the key in mcpServers) */
  serverName: string
  /** Transport type */
  transport: 'stdio' | 'sse' | 'streamable_http'
  /** For stdio: the command to run (e.g. "npx -y @org/server") */
  command?: string
  /** For stdio: arguments to pass */
  args?: string[]
  /** For remote: the endpoint URL */
  url?: string
  /** Optional env vars the server needs */
  envVars?: Record<string, string>
}

function stdioBlock(p: MCPConfigParams): object {
  const entry: Record<string, unknown> = {
    command: p.command ?? '',
    args: p.args ?? [],
  }
  if (p.envVars && Object.keys(p.envVars).length > 0) {
    entry.env = p.envVars
  }
  return entry
}

function remoteBlock(p: MCPConfigParams): object {
  return {
    url: p.url ?? '',
    ...(p.envVars && Object.keys(p.envVars).length > 0 ? { env: p.envVars } : {}),
  }
}

function configBlock(p: MCPConfigParams): object {
  return p.transport === 'stdio' ? stdioBlock(p) : remoteBlock(p)
}

function formatJson(obj: unknown): string {
  return JSON.stringify(obj, null, 2)
}

export const MCP_HOSTS: MCPHostConfig[] = [
  {
    name: 'Claude Desktop',
    configPath: '~/Library/Application Support/Claude/claude_desktop_config.json',
    generate: (p) =>
      formatJson({
        mcpServers: {
          [p.serverName]: configBlock(p),
        },
      }),
  },
  {
    name: 'Claude Code',
    configPath: '~/.claude.json',
    generate: (p) =>
      formatJson({
        mcpServers: {
          [p.serverName]: configBlock(p),
        },
      }),
  },
  {
    name: 'Cursor',
    configPath: '.cursor/mcp.json',
    generate: (p) =>
      formatJson({
        mcpServers: {
          [p.serverName]: configBlock(p),
        },
      }),
  },
  {
    name: 'Windsurf',
    configPath: '~/.codeium/windsurf/mcp_config.json',
    generate: (p) =>
      formatJson({
        mcpServers: {
          [p.serverName]: configBlock(p),
        },
      }),
  },
  {
    name: 'VS Code',
    configPath: '.vscode/mcp.json',
    generate: (p) => {
      // VS Code uses "inputs" array and "servers" object
      const server: Record<string, unknown> = p.transport === 'stdio'
        ? { command: p.command ?? '', args: p.args ?? [] }
        : { url: p.url ?? '' }
      if (p.envVars && Object.keys(p.envVars).length > 0) {
        server.env = p.envVars
      }
      return formatJson({
        servers: {
          [p.serverName]: server,
        },
      })
    },
  },
]

/**
 * Parse a package entry from the API into MCPConfigParams.
 */
export function packageToConfigParams(
  serverName: string,
  pkg: {
    registryType: string
    identifier: string
    version: string
    transport: { type: string; url?: string }
  },
): MCPConfigParams {
  const transport = pkg.transport.type as MCPConfigParams['transport']

  if (transport === 'stdio') {
    // Build command + args based on registry type
    switch (pkg.registryType) {
      case 'npm':
        return {
          serverName,
          transport: 'stdio',
          command: 'npx',
          args: ['-y', `${pkg.identifier}@${pkg.version}`],
        }
      case 'pypi':
        return {
          serverName,
          transport: 'stdio',
          command: 'uvx',
          args: [`${pkg.identifier}==${pkg.version}`],
        }
      case 'docker':
        return {
          serverName,
          transport: 'stdio',
          command: 'docker',
          args: ['run', '-i', '--rm', `${pkg.identifier}:${pkg.version}`],
        }
      case 'cargo':
        return {
          serverName,
          transport: 'stdio',
          command: 'cargo',
          args: ['run', '--package', pkg.identifier],
        }
      case 'go':
        return {
          serverName,
          transport: 'stdio',
          command: 'go',
          args: ['run', `${pkg.identifier}@v${pkg.version}`],
        }
      default:
        return {
          serverName,
          transport: 'stdio',
          command: pkg.identifier,
          args: [],
        }
    }
  }

  // Remote transports
  return {
    serverName,
    transport,
    url: pkg.transport.url ?? '',
  }
}
