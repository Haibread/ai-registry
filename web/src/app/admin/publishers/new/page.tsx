"use client"

import { useActionState } from "react"
import Link from "next/link"
import { ArrowLeft, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { createPublisher, type CreatePublisherState } from "../actions"

const initialState: CreatePublisherState = {}

export default function NewPublisherPage() {
  const [state, formAction, isPending] = useActionState(createPublisher, initialState)

  return (
    <div className="space-y-6 max-w-lg mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link href="/admin/publishers" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Publishers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Publisher</span>
      </nav>

      <h1 className="text-2xl font-bold">New Publisher</h1>

      {state.error && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <p>{state.error}</p>
        </div>
      )}

      <form action={formAction}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Publisher Details</CardTitle>
            <CardDescription>Publishers are namespaces for MCP servers and agents.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="slug">
                Slug <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input
                id="slug"
                name="slug"
                placeholder="my-org"
                pattern="^[a-z0-9-]+"
                title="Lowercase letters, numbers, and hyphens only"
                required
                aria-required="true"
              />
              <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only.</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="name">
                Name <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input id="name" name="name" placeholder="My Organization" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="contact">Contact email</Label>
              <Input id="contact" name="contact" type="email" placeholder="team@example.com" />
            </div>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full mt-6" disabled={isPending}>
          {isPending ? "Creating…" : "Create Publisher"}
        </Button>
      </form>
    </div>
  )
}
