import { headers } from "next/headers"
import { auth } from "@/auth"
import { AdminSidebar } from "@/components/layout/admin-sidebar"
import Link from "next/link"
import { signOut } from "@/auth"
import { Button } from "@/components/ui/button"

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const session = await auth()
  const headersList = await headers()
  const pathname = headersList.get("x-invoke-path") ?? ""

  return (
    <div className="flex min-h-screen flex-col">
      {/* Admin top bar */}
      <header className="sticky top-0 z-50 border-b bg-background h-14 flex items-center px-6 gap-4">
        <Link href="/" className="flex items-center gap-2 font-semibold text-sm">
          <div className="flex h-6 w-6 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">
            AI
          </div>
          Registry
        </Link>
        <span className="text-muted-foreground text-sm">/</span>
        <span className="text-sm font-medium">Admin</span>
        <div className="ml-auto flex items-center gap-3">
          <span className="text-sm text-muted-foreground hidden sm:block">
            {session?.user?.email}
          </span>
          <form
            action={async () => {
              "use server"
              await signOut({ redirectTo: "/" })
            }}
          >
            <Button variant="ghost" size="sm" type="submit">
              Sign out
            </Button>
          </form>
        </div>
      </header>

      <div className="flex flex-1">
        <AdminSidebar pathname={pathname} />
        <main className="flex-1 p-6 overflow-auto">{children}</main>
      </div>
    </div>
  )
}
