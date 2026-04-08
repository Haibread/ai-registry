import type { Metadata } from "next"
import { getApiClient } from "@/lib/api-client"
import { NewAgentForm } from "./form"

export const metadata: Metadata = { title: "New Agent" }

export default async function NewAgentPage() {
  const api = await getApiClient()
  const { data: pubData } = await api.GET("/api/v1/publishers", {
    params: { query: { limit: 100 } },
  })
  const publishers = pubData?.items ?? []

  return <NewAgentForm publishers={publishers} />
}
