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
      const isAdminRoute = nextUrl.pathname.startsWith("/admin")
      if (isAdminRoute) {
        // Reject if not logged in OR if the Keycloak token could not be refreshed.
        // Returning false triggers a redirect to the signIn page.
        return !!auth?.user && auth.error !== "RefreshAccessTokenError"
      }
      return true
    },
  },
} satisfies NextAuthConfig
