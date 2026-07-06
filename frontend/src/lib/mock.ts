import type { StateDTO } from './types'

// mockState renders the full UI in a plain browser (headless-Chrome dev/preview)
// where the Wails runtime is absent. It is never used inside the webview.
export const mockState: StateDTO = {
  server: 'https://api.klutch.lk',
  host: 'front-desk-pc',
  enrolled: true,
  connected: true,
  lastError: '',
  version: 'v1.1.0',
  availableVersion: '',
  lastCheck: new Date().toISOString(),
  autoUpdate: true,
  defaultPrinter: 'HP LaserJet Pro',
  theme: 'dark',
  notifyDone: true,
  notifyFailed: true,
  notifyWeekly: false,
  jobsOk: 128,
  jobsFailed: 3,
  printers: [
    { name: 'HP LaserJet Pro', model: 'HP M428 · Office', status: 'printing', stateReason: '', connection: 'Wi-Fi', location: 'Front · Office', queued: 3, default: true },
    { name: 'Brother QL-820', model: 'Label printer', status: 'idle', stateReason: '', connection: 'USB', location: 'Print desk', queued: 0, default: false },
    { name: 'Prusa MK4', model: '3D printer · Workshop', status: 'online', stateReason: '', connection: 'Cloud', location: 'Workshop', queued: 1, default: false },
    { name: 'Epson SureColor P900', model: 'Large format · Studio', status: 'online', stateReason: '', connection: 'Wi-Fi', location: 'Studio', queued: 0, default: false },
    { name: 'Star TSP143', model: 'Receipt · Register 2', status: 'error', stateReason: 'Out of paper', connection: 'USB', location: 'Register 2', queued: 2, default: false },
    { name: 'Canon PIXMA G620', model: 'Photo · Home office', status: 'error', stateReason: 'Toner low', connection: 'Wi-Fi', location: 'Home office', queued: 0, default: false },
  ],
  activeJobs: [
    { id: 'j100', printer: 'HP LaserJet Pro', doc: 'Q3-Report-Final.pdf', kind: 'pdf', state: 'printing', percent: 0.7 },
    { id: 'j101', printer: 'Prusa MK4', doc: 'bracket_v3.gcode', kind: 'pdf', state: 'printing', percent: 0.42 },
    { id: 'j102', printer: 'HP LaserJet Pro', doc: 'Poster-A1-launch.tiff', kind: 'pdf', state: 'queued', percent: -1 },
    { id: 'j103', printer: 'Brother QL-820', doc: 'invoices-June.pdf', kind: 'pdf', state: 'queued', percent: -1 },
    { id: 'j104', printer: 'Star TSP143', doc: 'Shipping-labels.pdf', kind: 'escpos_raster', state: 'paused', percent: -1 },
  ],
  recentJobs: [
    { id: 'h1', printer: 'HP LaserJet Pro', doc: 'Contract-signed.pdf', kind: 'pdf', status: 'ok', error: '', finishedAt: new Date(Date.now() - 3.6e6).toISOString() },
    { id: 'h2', printer: 'Star TSP143', doc: 'Receipt-8841', kind: 'escpos_raster', status: 'failed', error: 'Out of paper', finishedAt: new Date(Date.now() - 7.2e6).toISOString() },
    { id: 'h3', printer: 'Epson SureColor P900', doc: 'menu-A3.pdf', kind: 'pdf', status: 'ok', error: '', finishedAt: new Date(Date.now() - 1.1e7).toISOString() },
  ],
}
