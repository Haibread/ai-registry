import "@testing-library/jest-dom"

// ─────────────────────────────────────────────────────────────────────────────
// Radix Select / Popover / Dropdown shims for jsdom
//
// Radix primitives call PointerEvent APIs and Element.scrollIntoView on open.
// jsdom does not implement any of these, so without the shims below every
// test that opens a Radix Select throws "hasPointerCapture is not a function"
// or similar.
//
// Applied once at setup time (vitest.config.ts → setupFiles) rather than
// per-test in beforeEach, because they only need to exist, not reset. The
// `typeof Element` guard is required because vitest also runs pure-Node
// tests (e.g. `src/lib/utils.test.ts`) that don't load jsdom — touching
// `Element` there would throw at setup-file import time and fail every
// test in that file.
// ─────────────────────────────────────────────────────────────────────────────

if (typeof Element !== 'undefined') {
  type PatchableElement = Element & {
    hasPointerCapture?: (pointerId: number) => boolean
    releasePointerCapture?: (pointerId: number) => void
    scrollIntoView?: (arg?: boolean | ScrollIntoViewOptions) => void
  }

  const proto = Element.prototype as PatchableElement

  if (!proto.hasPointerCapture) {
    proto.hasPointerCapture = () => false
  }
  if (!proto.releasePointerCapture) {
    proto.releasePointerCapture = () => {}
  }
  if (!proto.scrollIntoView) {
    proto.scrollIntoView = () => {}
  }
}
