import type { StateDTO } from './types'
import { mockState } from './mock'

// Typed surface of the Go App bound by Wails (window.go.desktopapp.App). When the
// Wails runtime is absent (plain browser / headless-Chrome dev), we fall back to
// an in-memory mock so the whole UI renders and simple actions feel live.
interface AppBindings {
  GetState(): Promise<StateDTO>
  SetServer(url: string): Promise<void>
  Enroll(code: string): Promise<void>
  SetToken(token: string): Promise<void>
  SetDefaultPrinter(name: string): Promise<void>
  SetTheme(theme: string): Promise<void>
  SetAutoUpdate(on: boolean): Promise<void>
  SetNotifyDone(on: boolean): Promise<void>
  SetNotifyFailed(on: boolean): Promise<void>
  SetNotifyWeekly(on: boolean): Promise<void>
  PauseJob(id: string): Promise<void>
  ResumeJob(id: string): Promise<void>
  CancelJob(id: string): Promise<void>
  ReprintJob(id: string): Promise<void>
  PauseAll(): Promise<void>
  CheckForUpdate(): Promise<string>
  ApplyUpdate(): Promise<void>
  InstallApp(): Promise<string>
  UninstallApp(): Promise<void>
  IsAppInstalled(): Promise<boolean>
  SetAutostart(on: boolean): Promise<void>
  AutostartEnabled(): Promise<boolean>
}

declare global {
  interface Window {
    go?: { desktopapp: { App: AppBindings } }
    runtime?: { EventsOn(name: string, cb: (...data: unknown[]) => void): () => void }
  }
}

const bindings = (): AppBindings | null => window.go?.desktopapp?.App ?? null

/** True inside the Wails webview, false in a plain browser (mock mode). */
export const isWebview = (): boolean => bindings() !== null

// --- mock backing store (browser only) --------------------------------------

// A portable deep clone; structuredClone is unavailable in some older WebKitGTK
// builds, and this runs lazily (never at module load) so an unsupported runtime
// can't blank the whole app.
const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v)) as T

// Go serializes nil slices as null; coerce the list fields to arrays so the UI
// can always index them safely.
function normalize(s: StateDTO): StateDTO {
  if (!s.printers) s.printers = []
  if (!s.activeJobs) s.activeJobs = []
  if (!s.recentJobs) s.recentJobs = []
  return s
}

let local: StateDTO | null = null
const localState = (): StateDTO => (local ??= clone(mockState))
const listeners = new Set<(s: StateDTO) => void>()
const pushLocal = () => listeners.forEach((l) => l(clone(localState())))

/** Subscribe to state; returns an unsubscribe fn. Fires immediately with current. */
export function onState(cb: (s: StateDTO) => void): () => void {
  const app = bindings()
  if (app && window.runtime) {
    app.GetState().then((s) => cb(normalize(s))).catch(() => {})
    return window.runtime.EventsOn('state', (s) => cb(normalize(s as StateDTO)))
  }
  listeners.add(cb)
  cb(clone(localState()))
  return () => {
    listeners.delete(cb)
  }
}

export async function getState(): Promise<StateDTO> {
  const app = bindings()
  return app ? normalize(await app.GetState()) : clone(localState())
}

// call runs a bound method in the webview, or applies a mock mutation in-browser.
async function call<R = void>(
  fn: (a: AppBindings) => Promise<R>,
  mock?: (s: StateDTO) => void,
  mockResult?: R,
): Promise<R> {
  const app = bindings()
  if (app) return fn(app)
  if (mock) {
    mock(localState())
    pushLocal()
  }
  return mockResult as R
}

// --- actions -----------------------------------------------------------------

export const agent = {
  setServer: (url: string) => call((a) => a.SetServer(url), (s) => (s.server = url)),
  enroll: (code: string) => call((a) => a.Enroll(code), (s) => (s.enrolled = true)),
  setToken: (t: string) => call((a) => a.SetToken(t), (s) => (s.enrolled = true)),
  setDefaultPrinter: (name: string) =>
    call((a) => a.SetDefaultPrinter(name), (s) => {
      s.defaultPrinter = name
      s.printers.forEach((p) => (p.default = p.name === name))
    }),
  setTheme: (theme: string) => call((a) => a.SetTheme(theme), (s) => (s.theme = theme)),
  setAutoUpdate: (on: boolean) => call((a) => a.SetAutoUpdate(on), (s) => (s.autoUpdate = on)),
  setNotifyDone: (on: boolean) => call((a) => a.SetNotifyDone(on), (s) => (s.notifyDone = on)),
  setNotifyFailed: (on: boolean) => call((a) => a.SetNotifyFailed(on), (s) => (s.notifyFailed = on)),
  setNotifyWeekly: (on: boolean) => call((a) => a.SetNotifyWeekly(on), (s) => (s.notifyWeekly = on)),
  pauseJob: (id: string) =>
    call((a) => a.PauseJob(id), (s) => setJobState(s, id, 'paused')),
  resumeJob: (id: string) =>
    call((a) => a.ResumeJob(id), (s) => setJobState(s, id, 'printing')),
  cancelJob: (id: string) =>
    call((a) => a.CancelJob(id), (s) => (s.activeJobs = s.activeJobs.filter((j) => j.id !== id))),
  reprintJob: (id: string) => call((a) => a.ReprintJob(id)),
  pauseAll: () =>
    call((a) => a.PauseAll(), (s) => s.activeJobs.forEach((j) => (j.state = 'paused'))),
  checkForUpdate: () => call<string>((a) => a.CheckForUpdate(), undefined, ''),
  applyUpdate: () => call((a) => a.ApplyUpdate()),
  installApp: () => call<string>((a) => a.InstallApp(), undefined, ''),
  uninstallApp: () => call((a) => a.UninstallApp()),
  isAppInstalled: () => call<boolean>((a) => a.IsAppInstalled(), undefined, true),
  setAutostart: (on: boolean) => call((a) => a.SetAutostart(on)),
  autostartEnabled: () => call<boolean>((a) => a.AutostartEnabled(), undefined, false),
}

function setJobState(s: StateDTO, id: string, state: string) {
  const j = s.activeJobs.find((x) => x.id === id)
  if (j) j.state = state
}
