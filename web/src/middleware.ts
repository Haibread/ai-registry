import NextAuth from "next-auth"
import { authConfig } from "@/auth.config"

// Run the edge-compatible auth middleware only on admin routes.
export const { auth: middleware } = NextAuth(authConfig)

export const config = {
  matcher: ["/admin/:path*"],
}
