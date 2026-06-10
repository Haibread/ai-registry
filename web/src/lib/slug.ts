/**
 * Client-side mirror of the server's slug rule (lowercase letters, digits,
 * hyphens; max 63 chars). The server re-validates on write — this exists for
 * inline feedback before the round-trip.
 */

export const SLUG_MAX_LENGTH = 63
const SLUG_RE = /^[a-z0-9-]+$/

/** Validate a slug; null means valid. Emptiness is the `required` attribute's job. */
export function slugError(value: string): string | null {
  if (value === '') return null
  if (value.length > SLUG_MAX_LENGTH) return `Must be at most ${SLUG_MAX_LENGTH} characters.`
  if (!SLUG_RE.test(value)) return 'Use lowercase letters, numbers, and hyphens only.'
  return null
}
