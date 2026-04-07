import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/auth"

const API_URL = process.env.API_URL ?? "http://localhost:8081"

/**
 * Generic proxy for client components that need to call the backend.
 *
 * /api/proxy/mcp/servers?q=foo  →  http://backend:8081/api/v1/mcp/servers?q=foo
 *
 * The session access token is injected as a Bearer header when present,
 * so client components never handle tokens directly.
 */
async function handler(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
): Promise<NextResponse> {
  const { path } = await params
  const session = await auth()

  const upstream = new URL(`${API_URL}/api/v1/${path.join("/")}`)
  req.nextUrl.searchParams.forEach((value, key) => {
    upstream.searchParams.set(key, value)
  })

  const headers: HeadersInit = { "Content-Type": "application/json" }
  if (session?.accessToken) {
    headers["Authorization"] = `Bearer ${session.accessToken}`
  }

  const isReadMethod = req.method === "GET" || req.method === "HEAD"
  const body = isReadMethod ? undefined : await req.text()

  const resp = await fetch(upstream.toString(), {
    method: req.method,
    headers,
    body,
  })

  const text = await resp.text()
  return new NextResponse(text, {
    status: resp.status,
    headers: {
      "Content-Type": resp.headers.get("Content-Type") ?? "application/json",
    },
  })
}

export const GET = handler
export const POST = handler
export const PUT = handler
export const PATCH = handler
export const DELETE = handler
