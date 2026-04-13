/**
 * mode-labels.ts
 *
 * Maps MIME-style mode strings (used in A2A agent cards for
 * default_input_modes / default_output_modes) to human-friendly labels
 * with short descriptions.
 */

export interface ModeInfo {
  label: string
  description: string
}

const MODE_MAP: Record<string, ModeInfo> = {
  'text/plain': {
    label: 'Plain text',
    description: 'Unformatted text messages',
  },
  'text/markdown': {
    label: 'Markdown',
    description: 'Formatted text with Markdown syntax',
  },
  'text/html': {
    label: 'HTML',
    description: 'Structured HTML content',
  },
  'application/json': {
    label: 'JSON',
    description: 'Structured JSON data',
  },
  'image/png': {
    label: 'PNG image',
    description: 'Raster image in PNG format',
  },
  'image/jpeg': {
    label: 'JPEG image',
    description: 'Raster image in JPEG format',
  },
  'image/*': {
    label: 'Images',
    description: 'Any image format',
  },
  'audio/*': {
    label: 'Audio',
    description: 'Audio content (speech, sound)',
  },
  'video/*': {
    label: 'Video',
    description: 'Video content',
  },
  'application/pdf': {
    label: 'PDF',
    description: 'PDF documents',
  },
}

/**
 * Get human-friendly info for a mode string.
 * Returns undefined for unknown modes.
 */
export function getModeInfo(mode: string): ModeInfo | undefined {
  return MODE_MAP[mode]
}

/**
 * Get a short label for a mode, falling back to the raw mode string.
 */
export function getModeLabel(mode: string): string {
  return MODE_MAP[mode]?.label ?? mode
}
