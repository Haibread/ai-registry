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

// ─────────────────────────────────────────────────────────────────────────────
// localStorage polyfill
//
// Node 22+ ships an experimental built-in `globalThis.localStorage` that, when
// the runtime is started without a valid `--localstorage-file`, is a broken stub
// whose methods are not functions. On our test runner (Node 25) it shadows
// jsdom's Storage, so any code calling `localStorage.getItem/setItem/clear`
// (PublisherContext, the auth token store, …) throws "is not a function".
//
// Install a clean in-memory Storage on both globalThis and window so bare
// `localStorage` works deterministically regardless of the Node build. Force it
// via defineProperty to override the read-only built-in / jsdom getter.
// ─────────────────────────────────────────────────────────────────────────────

class MemoryStorage implements Storage {
  private store = new Map<string, string>()
  get length(): number {
    return this.store.size
  }
  clear(): void {
    this.store.clear()
  }
  getItem(key: string): string | null {
    return this.store.has(key) ? (this.store.get(key) as string) : null
  }
  key(index: number): string | null {
    return Array.from(this.store.keys())[index] ?? null
  }
  removeItem(key: string): void {
    this.store.delete(key)
  }
  setItem(key: string, value: string): void {
    this.store.set(key, String(value))
  }
}

{
  const storage = new MemoryStorage()
  Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true, writable: true })
  if (typeof window !== 'undefined') {
    Object.defineProperty(window, 'localStorage', { value: storage, configurable: true, writable: true })
  }
}
