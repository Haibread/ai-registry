import type { Metadata } from "next"
import { redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getApiClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "New Publisher" }

export default function NewPublisherPage() {
  async function create(formData: FormData) {
    "use server"
    const api = await getApiClient()
    const { error } = await api.POST("/api/v1/publishers", {
      body: {
        slug: formData.get("slug") as string,
        name: formData.get("name") as string,
        contact: (formData.get("contact") as string) || undefined,
      },
    })
    if (!error) redirect("/admin/publishers")
  }

  return (
    <div className="space-y-6 max-w-lg">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/publishers" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold">New Publisher</h1>
      </div>

      <form action={create} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="slug">Slug *</Label>
          <Input
            id="slug"
            name="slug"
            placeholder="my-org"
            pattern="^[a-z0-9-]+"
            required
          />
          <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only.</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="name">Name *</Label>
          <Input id="name" name="name" placeholder="My Organization" required />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="contact">Contact</Label>
          <Input id="contact" name="contact" type="email" placeholder="team@example.com" />
        </div>

        <Button type="submit" className="w-full">
          Create Publisher
        </Button>
      </form>
    </div>
  )
}
