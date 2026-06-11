/**
 * Instance-wide tag vocabulary (GET /api/v1/tags).
 *
 * The endpoint is public and returns the FULL vocabulary — deactivated tags
 * included — because versions published while a tag was active keep carrying
 * its slug and still need display resolution. Pickers must offer only the
 * active tags (`activeInstanceTags`).
 */

import { useQuery } from '@tanstack/react-query'
import { getPublicClient } from '@/lib/api-client'
import type { components } from '@/lib/schema'

export type InstanceTag = components['schemas']['InstanceTag']

export function useInstanceTags() {
  return useQuery({
    queryKey: ['instance-tags'],
    queryFn: async () => {
      // The operation declares only a 200, so openapi-fetch types `error` as
      // never — a missing body is the only failure signal to branch on.
      const r = await getPublicClient().GET('/api/v1/tags')
      if (!r.data) throw new Error('Failed to load tags.')
      return r.data
    },
    // The vocabulary changes rarely (Server-Admin curation); cards, detail
    // pages, and filters all resolve through this one cache entry.
    staleTime: 5 * 60_000,
  })
}

/** Builds a slug → tag lookup for resolving the slugs carried by entries/versions. */
export function indexInstanceTags(tags: InstanceTag[] | undefined): Map<string, InstanceTag> {
  return new Map((tags ?? []).map((t) => [t.slug, t]))
}

/** The tags publishers may tick on a new version. */
export function activeInstanceTags(tags: InstanceTag[] | undefined): InstanceTag[] {
  return (tags ?? []).filter((t) => t.active)
}

/** Field-name prefix the InstanceTagPicker checkboxes use inside a form. */
export const INSTANCE_TAG_FIELD_PREFIX = 'instance_tag_'

/** Collects the slugs ticked by an InstanceTagPicker from the submitted form. */
export function collectInstanceTags(fd: FormData): string[] {
  return [...fd.keys()]
    .filter((k) => k.startsWith(INSTANCE_TAG_FIELD_PREFIX))
    .map((k) => k.slice(INSTANCE_TAG_FIELD_PREFIX.length))
}
