import type { DefaultSession } from "next-auth"

declare module "next-auth" {
  interface Session extends DefaultSession {
    /** Keycloak access token forwarded to the backend as a Bearer token. */
    accessToken?: string
    /**
     * Set to "RefreshAccessTokenError" when the Keycloak refresh token has
     * expired or been revoked. The UI should force a re-login when this is set.
     */
    error?: "RefreshAccessTokenError"
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string
    /** Absolute epoch ms at which the access token expires. */
    expiresAt?: number
    refreshToken?: string
    error?: "RefreshAccessTokenError"
  }
}
