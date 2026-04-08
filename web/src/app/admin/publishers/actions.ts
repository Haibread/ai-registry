"use server"

import { redirect } from "next/navigation"
import { getApiClient } from "@/lib/api-client"

export type CreatePublisherState = { error?: string }

export async function createPublisher(
  _prev: CreatePublisherState,
  formData: FormData
): Promise<CreatePublisherState> {
  const slug = (formData.get("slug") as string).trim()
  const name = (formData.get("name") as string).trim()

  if (!slug || !name) {
    return { error: "Slug and name are required." }
  }

  const api = await getApiClient()
  const { error } = await api.POST("/api/v1/publishers", {
    body: {
      slug,
      name,
      contact: (formData.get("contact") as string) || undefined,
    },
  })

  if (error) {
    const msg = (error as { title?: string } | undefined)?.title
    return { error: msg ?? "Failed to create publisher. The slug may already be in use." }
  }

  redirect("/admin/publishers")
}
