import { Link } from 'react-router-dom'

export function Footer() {
  return (
    <footer className="border-t py-6 text-center text-sm text-muted-foreground">
      <div className="container flex flex-col gap-2 sm:flex-row sm:justify-between sm:items-center">
        <span>AI Registry — MCP servers &amp; AI agents, all in one place.</span>
        <nav className="flex gap-4">
          <Link to="/getting-started" className="hover:text-foreground transition-colors">
            Getting started
          </Link>
          <Link to="/changelog" className="hover:text-foreground transition-colors">
            Changelog
          </Link>
        </nav>
      </div>
    </footer>
  )
}
