import type { Metadata } from "next"
import { getApiClient } from "@/lib/api-client"
import { NewMCPServerForm } from "./form"

export const metadata: Metadata = { title: "New MCP Server" }

export default async function NewMCPServerPage() {
  const api = await getApiClient()
  const { data: pubData } = await api.GET("/api/v1/publishers", {
    params: { query: { limit: 100 } },
  })
  const publishers = pubData?.items ?? []

  return <NewMCPServerForm publishers={publishers} />
}
