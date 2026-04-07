import type { Metadata } from "next"
import { redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getApiClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "New Agent" }

export default async function NewAgentPage() {
  const api = await getApiClient()
  const { data: pubData } = await api.GET("/api/v1/publishers", {
    params: { query: { limit: 100 } },
  })
  const publishers = pubData?.items ?? []

  async function create(formData: FormData) {
    "use server"
    const client = await getApiClient()
    const { error } = await client.POST("/api/v1/agents", {
      body: {
        namespace: formData.get("namespace") as string,
        slug: formData.get("slug") as string,
        name: formData.get("name") as string,
        description: (formData.get("description") as string) || undefined,
      },
    })
    if (!error) redirect("/admin/agents")
  }

  return (
    <div className="space-y-6 max-w-lg">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/agents" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold">New Agent</h1>
      </div>

      <form action={create} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="namespace">Namespace (publisher) *</Label>
          <select
            id="namespace"
            name="namespace"
            required
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="">Select publisher…</option>
            {publishers.map((p) => (
              <option key={p.id} value={p.slug}>
                {p.slug} — {p.name}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="slug">Slug *</Label>
          <Input id="slug" name="slug" placeholder="my-agent" pattern="^[a-z0-9-]+" required />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="name">Name *</Label>
          <Input id="name" name="name" placeholder="My Agent" required />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="description">Description</Label>
          <Input id="description" name="description" placeholder="What this agent does…" />
        </div>

        <Button type="submit" className="w-full">
          Create Agent
        </Button>
      </form>
    </div>
  )
}
