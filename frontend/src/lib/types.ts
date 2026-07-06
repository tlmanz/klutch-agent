// View models mirroring internal/desktopapp DTOs (Go → JSON → TS).

export interface PrinterDTO {
  name: string
  model: string
  status: 'online' | 'idle' | 'printing' | 'error' | 'offline' | string
  stateReason: string
  connection: 'Wi-Fi' | 'USB' | 'Cloud' | string
  location: string
  queued: number
  default: boolean
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
