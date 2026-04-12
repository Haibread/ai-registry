/**
 * GettingStartedPage — onboarding walkthrough.
 *
 * Structured as five uniformly-sized step cards rather than a flat markdown
 * dump so every section has the same visual weight.
 */

import { Link } from 'react-router-dom'
import { Search, FileText, Download, CheckCircle2, Compass, ExternalLink } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface Step {
  number: number
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
  body: React.ReactNode
}

const STEPS: Step[] = [
  {
    number: 1,
    icon: Search,
    title: 'Find a Server or Agent',
    description: 'Browse and search the full catalog.',
    body: (
      <div className="space-y-2">
        <p>
          Use the <Link to="/explore" className="text-primary hover:underline">Explore</Link> page
          to browse and search the full catalog. You can filter by:
        </p>
        <ul className="list-disc pl-5 space-y-1">
          <li><strong>Type</strong> — MCP servers or AI agents</li>
          <li><strong>Transport</strong> — stdio, SSE, or streamable HTTP</li>
          <li><strong>Tags</strong> — categories like <code>database</code>, <code>code</code>, <code>search</code></li>
          <li><strong>Publisher</strong> — the organization that maintains the entry</li>
        </ul>
      </div>
    ),
  },
  {
    number: 2,
    icon: FileText,
    title: 'Review the Detail Page',
    description: "Check that the entry meets your needs before you install it.",
    body: (
      <ul className="list-disc pl-5 space-y-1">
        <li><strong>Status</strong> — is it <code>published</code> and <code>public</code>?</li>
        <li><strong>Freshness</strong> — when was the latest version published?</li>
        <li><strong>Capabilities</strong> — what tools and resources does it expose?</li>
        <li><strong>README</strong> — does the author provide usage documentation?</li>
      </ul>
    ),
  },
  {
    number: 3,
    icon: Download,
    title: 'Install or Connect',
    description: 'Install an MCP server or connect to an A2A agent.',
    body: (
      <div className="space-y-4">
        <div>
          <p className="font-medium text-foreground mb-2">MCP Servers</p>
          <p className="mb-2">
            Go to the <strong>Installation</strong> tab on the detail page to find the run command
            and a ready-to-paste config snippet for your host. Supported hosts:
          </p>
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-xs">
              <thead className="bg-muted/50">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">Host</th>
                  <th className="px-3 py-2 text-left font-medium">Config file</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr><td className="px-3 py-1.5">Claude Desktop</td><td className="px-3 py-1.5 font-mono">claude_desktop_config.json</td></tr>
                <tr><td className="px-3 py-1.5">Claude Code</td><td className="px-3 py-1.5 font-mono">.claude/settings.json</td></tr>
                <tr><td className="px-3 py-1.5">Cursor</td><td className="px-3 py-1.5 font-mono">.cursor/mcp.json</td></tr>
                <tr><td className="px-3 py-1.5">Windsurf</td><td className="px-3 py-1.5 font-mono">~/.windsurf/mcp_config.json</td></tr>
                <tr><td className="px-3 py-1.5">VS Code</td><td className="px-3 py-1.5 font-mono">.vscode/mcp.json</td></tr>
              </tbody>
            </table>
          </div>
        </div>
        <div>
          <p className="font-medium text-foreground mb-2">AI Agents (A2A)</p>
          <p>
            Go to the <strong>Connect</strong> tab to find the endpoint URL, auth requirements, and
            ready-to-use code snippets in curl, Python, TypeScript, and Go.
          </p>
        </div>
      </div>
    ),
  },
  {
    number: 4,
    icon: CheckCircle2,
    title: 'Verify It Works',
    description: 'Confirm your installation is live.',
    body: (
      <ul className="list-disc pl-5 space-y-1">
        <li>
          <strong>MCP servers</strong>: after adding the config, your host should list the
          server's tools and resources. Try invoking one.
        </li>
        <li>
          <strong>AI agents</strong>: send a <code>tasks/send</code> JSON-RPC request to the
          endpoint — you should get back a task object.
        </li>
      </ul>
    ),
  },
  {
    number: 5,
    icon: Compass,
    title: 'Explore More',
    description: 'Go deeper once you have the basics working.',
    body: (
      <ul className="list-disc pl-5 space-y-1">
        <li>
          Browse <Link to="/explore" className="text-primary hover:underline">publishers</Link>
          {' '}to discover trusted organizations.
        </li>
        <li>Check the <strong>Versions</strong> tab to see release history and compare diffs.</li>
        <li>Use the <strong>JSON</strong> tab to inspect the raw API data.</li>
      </ul>
    ),
  },
]

export default function GettingStartedPage() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'Getting Started' },
          ]}
        />

        <div className="space-y-2">
          <h1 className="text-3xl font-bold tracking-tight">Getting Started with the AI Registry</h1>
          <p className="text-muted-foreground">
            This guide walks you through discovering, configuring, and using MCP servers and AI
            agents from the registry.
          </p>
        </div>

        <div className="grid gap-4">
          {STEPS.map((step) => {
            const Icon = step.icon
            return (
              <Card key={step.number} className="h-full">
                <CardHeader className="pb-3">
                  <CardTitle className="flex items-center gap-3 text-lg">
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                      {step.number}
                    </span>
                    <Icon className="h-4 w-4 text-muted-foreground shrink-0" aria-hidden="true" />
                    <span>{step.title}</span>
                  </CardTitle>
                  <CardDescription>{step.description}</CardDescription>
                </CardHeader>
                <CardContent className="text-sm text-muted-foreground">{step.body}</CardContent>
              </Card>
            )
          })}
        </div>

        <Card>
          <CardContent className="py-4 text-sm text-muted-foreground flex flex-wrap items-center justify-between gap-3">
            <span>Need help with the underlying protocols?</span>
            <div className="flex gap-3">
              <a
                href="https://modelcontextprotocol.io/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-primary hover:underline"
              >
                MCP spec <ExternalLink className="h-3 w-3" />
              </a>
              <a
                href="https://a2a-protocol.org/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-primary hover:underline"
              >
                A2A protocol <ExternalLink className="h-3 w-3" />
              </a>
            </div>
          </CardContent>
        </Card>
      </main>
      <Footer />
    </div>
  )
}
