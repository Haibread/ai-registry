import { cn } from '@/lib/utils'

interface LogoMarkProps {
  className?: string
}

/** The registry's logo mark. Strokes are heavier than in `logo.svg` so the
 *  frame and the centre square stay legible at header size (~24px tall). */
export function LogoMark({ className }: LogoMarkProps) {
  return (
    <svg
      viewBox="-196 -128 392 256"
      className={cn('h-6 w-auto shrink-0', className)}
      aria-hidden="true"
      focusable="false"
    >
      <g stroke="#178BC9" fill="none">
        <path d="M-192 0H-104M104 0H192" strokeWidth="12" strokeLinecap="round" />
        <rect x="-104" y="-102" width="208" height="204" strokeWidth="16" />
      </g>
      <rect x="-24" y="-24" width="48" height="48" fill="none" stroke="#F2861F" strokeWidth="17" />
      <g fill="#178BC9">
        <circle cx="-104" r="27" />
        <circle cx="104" r="27" />
      </g>
      <g fill="#F05333">
        <circle cy="-102" r="27" />
        <circle cy="102" r="27" />
      </g>
    </svg>
  )
}
