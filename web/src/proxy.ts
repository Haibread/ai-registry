import NextAuth from "next-auth"
import { authConfig } from "@/auth.config"

// Protect admin routes by verifying the session.
// Next.js 16 runs proxy.ts in the Node.js runtime (not edge).
// authConfig is kept edge-compatible for forward-compat, but the Node.js
// runtime is fine here — it unlocks using the full auth.ts in the future.
const { auth } = NextAuth(authConfig)

export const proxy = auth

export const config = {
  matcher: ["/admin/:path*"],
}
