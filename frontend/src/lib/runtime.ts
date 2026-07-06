// Thin wrappers over the Wails JS runtime (window.runtime), all no-ops in a
// plain browser so the mock UI still works.

interface WailsRuntime {
  WindowSetSize?(w: number, h: number): void
  WindowSetMinSize?(w: number, h: number): void
  WindowShow?(): void
  WindowCenter?(): void
  EventsOn?(name: string, cb: (...data: unknown[]) => void): () => void
}

const rt = (): WailsRuntime | undefined => (window as unknown as { runtime?: WailsRuntime }).runtime

export function windowSetSize(w: number, h: number) {
  rt()?.WindowSetSize?.(w, h)
  rt()?.WindowCenter?.()
}

export function windowShow() {
  rt()?.WindowShow?.()
}

/** Subscribe to a Wails backend event; returns an unsubscribe fn. */
export function onEvent(name: string, cb: (...data: unknown[]) => void): () => void {
  return rt()?.EventsOn?.(name, cb) ?? (() => {})
}
