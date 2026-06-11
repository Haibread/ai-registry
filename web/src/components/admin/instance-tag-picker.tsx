/**
 * InstanceTagPicker — checkbox group over the ACTIVE instance-tag vocabulary
 * (the Server-Admin-curated list at GET /api/v1/tags), for the version
 * authoring forms. Ticked slugs freeze with the published version.
 *
 * Renders nothing when the vocabulary has no active tags, so instances that
 * don't curate tags don't show an empty fieldset. The parent form collects
 * the selection with `collectInstanceTags(formData)`.
 */

import { Checkbox } from '@/components/ui/checkbox'
import { TagBadge } from '@/components/ui/tag-badge'
import {
  INSTANCE_TAG_FIELD_PREFIX,
  activeInstanceTags,
  useInstanceTags,
} from '@/lib/use-instance-tags'

interface InstanceTagPickerProps {
  /** Slugs pre-ticked, e.g. carried over from the previous version. */
  defaultSelected?: string[]
}

export function InstanceTagPicker({ defaultSelected = [] }: InstanceTagPickerProps) {
  const { data } = useInstanceTags()
  const tags = activeInstanceTags(data?.items)
  if (tags.length === 0) return null

  return (
    <fieldset className="space-y-2 rounded-md border p-3">
      <legend className="px-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        Tags (optional)
      </legend>
      <p className="text-xs text-muted-foreground">
        Instance-wide markers curated by the registry admins. The selection is
        frozen with the published version.
      </p>
      <div className="flex flex-wrap gap-x-4 gap-y-2">
        {tags.map((t) => (
          // title on the whole label (not just the badge) so the tag's
          // description shows wherever the author hovers the row.
          <label
            key={t.slug}
            title={t.description || undefined}
            className="flex cursor-pointer items-center gap-2 text-sm"
          >
            <Checkbox
              name={`${INSTANCE_TAG_FIELD_PREFIX}${t.slug}`}
              defaultChecked={defaultSelected.includes(t.slug)}
            />
            <TagBadge slug={t.slug} tag={t} />
          </label>
        ))}
      </div>
    </fieldset>
  )
}
