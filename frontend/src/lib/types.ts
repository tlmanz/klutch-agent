// View models mirroring internal/desktopapp DTOs (Go → JSON → TS).

export interface PrinterDTO {
  name: string
  model: string
  raw: boolean // pass-through queue (receipt printer): prints dots, not pages
  status: 'online' | 'idle' | 'printing' | 'error' | 'offline' | string
  stateReason: string
  connection: 'Wi-Fi' | 'USB' | 'Cloud' | string
  location: string
  queued: number
  default: boolean
  // A stand-in row shown when this PC has no printers at all - not a queue, so no
  // action that talks to the spooler (remove, set default, print) applies to it.
  // Optional: the Go side always sends it, the browser mocks have no use for it.
  placeholder?: boolean
}

// A printer device the OS can reach, listed by the Add-printer dialog. Devices
// with `installed` already have a queue.
export interface DeviceDTO {
  uri: string
  name: string // suggested queue name
  info: string
  makeModel: string
  connection: 'Wi-Fi' | 'USB' | 'Cloud' | string
  driver: string // 'raw' | 'everywhere'
  installed: boolean
  queue: string
}

// One local-print request. The same object drives the preview and the print, so
// what the preview shows is what the printer receives.
export interface PrintOptionsDTO {
  path: string
  printer: string
  mode: 'color' | 'gray' | 'mono'
  dither: boolean
  threshold: number // 1..254
  rotate: number // 0 | 90 | 180 | 270
  invert: boolean
  widthPx: number // receipt head width in dots
  copies: number
  cut: boolean
  tearOffMm: number // blank feed after the image; -1 = use the printer's saved value
  align: number // 0 left, 1 centre, 2 right
  media: string // A4, Letter, … ('' = printer default)
  fitToPage: boolean
}

export interface PreviewDTO {
  dataUrl: string
  width: number
  height: number
  srcWidth: number
  srcHeight: number
  format: string
  printable: boolean
  image: boolean
  note: string
  raw: boolean
  tearOffMm: number
  tearOffPx: number
  lengthMm: number
}

export interface JobDTO {
  id: string
  printer: string
  doc: string
  kind: string
  state: 'queued' | 'printing' | 'paused' | string
  percent: number // -1 = indeterminate
}

export interface JobHistoryDTO {
  id: string
  printer: string
  doc: string
  kind: string
  status: 'ok' | 'failed' | string
  error: string
  finishedAt: string // RFC3339
}

export interface StateDTO {
  server: string
  host: string
  enrolled: boolean
  connected: boolean
  lastError: string
  version: string
  availableVersion: string
  lastCheck: string
  autoUpdate: boolean
  defaultPrinter: string
  theme: 'dark' | 'light' | string
  notifyDone: boolean
  notifyFailed: boolean
  notifyWeekly: boolean
  jobsOk: number
  jobsFailed: number
  printers: PrinterDTO[]
  activeJobs: JobDTO[]
  recentJobs: JobHistoryDTO[]
}
