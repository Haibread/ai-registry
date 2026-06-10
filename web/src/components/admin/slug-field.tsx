/**
 * SlugField — shared slug input for the admin create forms.
 *
 * Centralizes two fixes (UI/UX review P1.2):
 *  - The old inline `pattern="^[a-z0-9-]+"` fails to compile under the `v`
 *    regex flag Chromium applies to the HTML pattern attribute, which made the
 *    browser skip validation entirely. The pattern below escapes the hyphen
 *    (and drops the redundant anchor — HTML patterns are implicitly anchored).
 *  - None of the forms had inline validation; errors only surfaced after a
 *    server round-trip. This validates on blur with `aria-invalid` and a
 *    visible message, mirroring the server's slug rule (lowercase letters,
 *    digits, hyphens; max 63 chars).
 */

import { useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { slugError, SLUG_MAX_LENGTH } from '@/lib/slug'

interface SlugFieldProps {
  id?: string
  name?: string
  label?: string
  placeholder: string
}

export function SlugField({ id = 'slug', name = 'slug', label = 'Slug', placeholder }: SlugFieldProps) {
  const [error, setError] = useState<string | null>(null)

  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>
        {label} <span className="text-destructive" aria-hidden="true">*</span>
      </Label>
      <Input
        id={id}
        name={name}
        placeholder={placeholder}
        pattern="[a-z0-9\-]+"
        maxLength={SLUG_MAX_LENGTH}
        title="Lowercase letters, numbers, and hyphens only"
        required
        aria-required="true"
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : `${id}-hint`}
        onBlur={(e) => setError(slugError(e.currentTarget.value.trim()))}
        onChange={(e) => {
          // Once a field is marked invalid, re-validate as the user types so
          // the message clears the moment the value becomes acceptable.
          if (error) setError(slugError(e.currentTarget.value.trim()))
        }}
      />
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          Lowercase letters, numbers, and hyphens only.
        </p>
      )}
    </div>
  )
}
