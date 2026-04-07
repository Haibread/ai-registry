import createClient from "openapi-fetch"
import type { paths } from "./schema"
import { auth } from "@/auth"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

/**
 * Returns a server-side API client with the current session's access token
 * injected as a Bearer header. Must only be called from Server Components,
 * Route Handlers, or Server Actions.
 */
export async function getApiClient() {
  const session = await auth()
  const headers: Record<string, string> = {}
  if (session?.accessToken) {
    headers["Authorization"] = `Bearer ${session.accessToken}`
  }
  return createClient<paths>({ baseUrl: API_URL, headers })
}

/**
 * Unauthenticated client for public read endpoints.
 * Safe to call from any server context.
 */
export function getPublicClient() {
  return createClient<paths>({ baseUrl: API_URL })
}
