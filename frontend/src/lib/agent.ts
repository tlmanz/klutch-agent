import type { DeviceDTO, PreviewDTO, PrintOptionsDTO, StateDTO } from './types'
import { mockDevices, mockPreview, mockState } from './mock'

// Typed surface of the Go App bound by Wails (window.go.desktopapp.App). When the
// Wails runtime is absent (plain browser / headless-Chrome dev), we fall back to
// an in-memory mock so the whole UI renders and simple actions feel live.
interface AppBindings {
  GetState(): Promise<StateDTO>
  SetServer(url: string): Promise<void>
  Enroll(code: string): Promise<void>
  SetToken(token: string): Promise<void>
  Reconnect(): Promise<void>
  Disconnect(): Promise<void>
  SetDefaultPrinter(name: string): Promise<void>
  DiscoverDevices(): Promise<DeviceDTO[]>
  AddPrinter(name: string, uri: string, driver: string): Promise<void>
  RemovePrinter(name: string): Promise<void>
  PickFile(): Promise<string>
  TearOffMM(printer: string): Promise<number>
  SetTearOffMM(printer: string, mm: number): Promise<void>
  CutEnabled(printer: string): Promise<boolean>
  SetCutEnabled(printer: string, on: boolean): Promise<void>
  PreviewLocalFile(o: PrintOptionsDTO): Promise<PreviewDTO>
  PrintLocalFile(o: PrintOptionsDTO): Promise<string>
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
  reconnect: () => call((a) => a.Reconnect(), (s) => (s.connected = true)),
  disconnect: () =>
    call((a) => a.Disconnect(), (s) => {
      s.enrolled = false
      s.connected = false
      s.lastError = ''
    }),
  setDefaultPrinter: (name: string) =>
    call((a) => a.SetDefaultPrinter(name), (s) => {
      s.defaultPrinter = name
      s.printers.forEach((p) => (p.default = p.name === name))
    }),
  discoverDevices: () => call<DeviceDTO[]>((a) => a.DiscoverDevices(), undefined, clone(mockDevices)),
  addPrinter: (name: string, uri: string, driver: string) =>
    call((a) => a.AddPrinter(name, uri, driver), (s) => {
      const dev = mockDevices.find((d) => d.uri === uri)
      s.printers.push({
        name,
        model: dev?.makeModel || dev?.info || 'Printer',
        raw: (dev?.driver ?? 'raw') === 'raw',
        status: 'online',
        stateReason: '',
        connection: dev?.connection || 'USB',
        location: '',
        queued: 0,
        default: false,
      })
    }),
  removePrinter: (name: string) =>
    call((a) => a.RemovePrinter(name), (s) => {
      s.printers = s.printers.filter((p) => p.name !== name)
    }),
  pickFile: () => call<string>((a) => a.PickFile(), undefined, '/home/demo/Pictures/receipt-logo.png'),
  tearOffMM: (printer: string) => call<number>((a) => a.TearOffMM(printer), undefined, 30),
  setTearOffMM: (printer: string, mm: number) => call((a) => a.SetTearOffMM(printer, mm)),
  cutEnabled: (printer: string) => call<boolean>((a) => a.CutEnabled(printer), undefined, false),
  setCutEnabled: (printer: string, on: boolean) => call((a) => a.SetCutEnabled(printer, on)),
  previewLocalFile: (o: PrintOptionsDTO) =>
    call<PreviewDTO>((a) => a.PreviewLocalFile(o), undefined, mockPreview(o)),
  printLocalFile: (o: PrintOptionsDTO) =>
    call<string>((a) => a.PrintLocalFile(o), (s) => {
      s.activeJobs.unshift({
        id: `local-${s.activeJobs.length}`,
        printer: o.printer,
        doc: o.path.split('/').pop() || 'file',
        kind: o.mode === 'mono' ? 'escpos_raster' : 'image',
        state: 'printing',
        percent: -1,
      })
    }, 'local-mock'),
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
