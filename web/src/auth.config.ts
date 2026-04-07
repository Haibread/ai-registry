import type { NextAuthConfig } from "next-auth"

// Edge-compatible auth config — no Node.js-only imports.
// Used by middleware to verify sessions without loading provider SDKs.
export const authConfig = {
  providers: [],
  pages: {
    signIn: "/api/auth/signin",
  },
  callbacks: {
    authorized({ auth, request: { nextUrl } }) {
      const isLoggedIn = !!auth?.user
      const isAdminRoute = nextUrl.pathname.startsWith("/admin")
      if (isAdminRoute) {
        // Returning false triggers a redirect to the signIn page.
        return isLoggedIn
      }
      return true
    },
  },
} satisfies NextAuthConfig
