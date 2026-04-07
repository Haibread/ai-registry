import Link from "next/link"
import { auth } from "@/auth"
import { Button } from "@/components/ui/button"
import { signIn, signOut } from "@/auth"
import { Server, Bot } from "lucide-react"

export async function Header() {
  const session = await auth()

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-14 items-center gap-6">
        <Link href="/" className="flex items-center gap-2 font-semibold">
          <div className="flex h-7 w-7 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">
            AI
          </div>
          <span>Registry</span>
        </Link>

        <nav className="flex items-center gap-1 text-sm">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/mcp" className="flex items-center gap-1.5">
              <Server className="h-4 w-4" />
              MCP Servers
            </Link>
          </Button>
          <Button variant="ghost" size="sm" asChild>
            <Link href="/agents" className="flex items-center gap-1.5">
              <Bot className="h-4 w-4" />
              Agents
            </Link>
          </Button>
        </nav>

        <div className="ml-auto flex items-center gap-2">
          {session ? (
            <>
              <Button variant="ghost" size="sm" asChild>
                <Link href="/admin">Admin</Link>
              </Button>
              <form
                action={async () => {
                  "use server"
                  await signOut({ redirectTo: "/" })
                }}
              >
                <Button variant="outline" size="sm" type="submit">
                  Sign out
                </Button>
              </form>
            </>
          ) : (
            <form
              action={async () => {
                "use server"
                await signIn("keycloak", { redirectTo: "/admin" })
              }}
            >
              <Button size="sm" type="submit">
                Sign in
              </Button>
            </form>
          )}
        </div>
      </div>
    </header>
  )
}
