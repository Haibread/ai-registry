import NextAuth from "next-auth"
import Keycloak from "next-auth/providers/keycloak"
import { authConfig } from "./auth.config"

/** Refresh a Keycloak access token using the stored refresh token. */
async function refreshKeycloakToken(refreshToken: string): Promise<{
  accessToken: string
  expiresAt: number
  refreshToken: string
}> {
  const issuer = process.env.AUTH_KEYCLOAK_ISSUER!
  const tokenUrl = `${issuer}/protocol/openid-connect/token`

  const response = await fetch(tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      client_id: process.env.AUTH_KEYCLOAK_ID!,
      client_secret: process.env.AUTH_KEYCLOAK_SECRET!,
      refresh_token: refreshToken,
    }),
  })

  const tokens = await response.json()
  if (!response.ok) throw tokens

  return {
    accessToken: tokens.access_token as string,
    // expires_in is in seconds; convert to absolute epoch ms
    expiresAt: Date.now() + (tokens.expires_in as number) * 1000,
    // Use the new refresh token if Keycloak rotated it, otherwise keep old one
    refreshToken: (tokens.refresh_token as string | undefined) ?? refreshToken,
  }
}

export const { handlers, auth, signIn, signOut } = NextAuth({
  ...authConfig,
  providers: [
    Keycloak({
      clientId: process.env.AUTH_KEYCLOAK_ID!,
      clientSecret: process.env.AUTH_KEYCLOAK_SECRET!,
      issuer: process.env.AUTH_KEYCLOAK_ISSUER,
    }),
  ],
  callbacks: {
    ...authConfig.callbacks,

    async jwt({ token, account }) {
      // On first sign-in, persist the Keycloak tokens and their expiry.
      if (account) {
        return {
          ...token,
          accessToken: account.access_token,
          expiresAt: account.expires_at
            ? account.expires_at * 1000  // OAuth gives seconds; convert to ms
            : Date.now() + (account.expires_in as number) * 1000,
          refreshToken: account.refresh_token,
          error: undefined,
        }
      }

      // Access token is still valid — nothing to do.
      if (Date.now() < (token.expiresAt as number)) {
        return token
      }

      // Access token expired — try to refresh.
      if (!token.refreshToken) {
        return { ...token, error: "RefreshAccessTokenError" as const }
      }

      try {
        const refreshed = await refreshKeycloakToken(token.refreshToken as string)
        return {
          ...token,
          accessToken: refreshed.accessToken,
          expiresAt: refreshed.expiresAt,
          refreshToken: refreshed.refreshToken,
          error: undefined,
        }
      } catch {
        // Refresh failed — Keycloak session is gone (logged out, token revoked, etc.)
        return { ...token, error: "RefreshAccessTokenError" as const }
      }
    },

    async session({ session, token }) {
      session.accessToken = token.accessToken as string | undefined
      // Propagate any token error to the session so the UI can react.
      session.error = token.error as string | undefined
      return session
    },
  },
})
