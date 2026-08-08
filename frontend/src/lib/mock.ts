import type { DeviceDTO, PreviewDTO, PrintOptionsDTO, StateDTO } from './types'

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
    { name: 'HP LaserJet Pro', model: 'HP M428 · Office', raw: false, status: 'printing', stateReason: '', connection: 'Wi-Fi', location: 'Front · Office', queued: 3, default: true },
    { name: 'Brother QL-820', model: 'Label printer', raw: true, status: 'idle', stateReason: '', connection: 'USB', location: 'Print desk', queued: 0, default: false },
    { name: 'Prusa MK4', model: '3D printer · Workshop', raw: false, status: 'online', stateReason: '', connection: 'Cloud', location: 'Workshop', queued: 1, default: false },
    { name: 'Epson SureColor P900', model: 'Large format · Studio', raw: false, status: 'online', stateReason: '', connection: 'Wi-Fi', location: 'Studio', queued: 0, default: false },
    { name: 'Star TSP143', model: 'Receipt · Register 2', raw: true, status: 'error', stateReason: 'Out of paper', connection: 'USB', location: 'Register 2', queued: 2, default: false },
    { name: 'Canon PIXMA G620', model: 'Photo · Home office', raw: false, status: 'error', stateReason: 'Toner low', connection: 'Wi-Fi', location: 'Home office', queued: 0, default: false },
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

// mockPreview stands in for the Go image pipeline in mock mode: an SVG that
// reacts to the colour mode and rotation so the print screen can be exercised in
// a plain browser. The real preview is a PNG rendered by internal/imaging.
export function mockPreview(o: PrintOptionsDTO): PreviewDTO {
  const mono = o.mode === 'mono'
  const gray = o.mode === 'gray'
  const ink = mono ? '#000' : gray ? '#4a4a4a' : '#d97706'
  const tint = mono ? '#000' : gray ? '#8a8a8a' : '#0ea5e9'
  const w = o.widthPx || 576
  const h = Math.round(w * 1.35)
  const tearMm = o.tearOffMm < 0 ? 30 : o.tearOffMm
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
    <rect width="100%" height="100%" fill="${o.invert ? '#000' : '#fff'}"/>
    <circle cx="${w / 2}" cy="${h * 0.28}" r="${w * 0.18}" fill="${tint}"/>
    <rect x="${w * 0.12}" y="${h * 0.52}" width="${w * 0.76}" height="${h * 0.04}" fill="${ink}"/>
    <rect x="${w * 0.12}" y="${h * 0.6}" width="${w * 0.55}" height="${h * 0.04}" fill="${ink}"/>
    <rect x="${w * 0.12}" y="${h * 0.68}" width="${w * 0.66}" height="${h * 0.04}" fill="${ink}"/>
    <text x="${w / 2}" y="${h * 0.88}" font-family="monospace" font-size="${w * 0.07}"
          text-anchor="middle" fill="${ink}">PREVIEW ${o.rotate}°</text>
  </svg>`
  return {
    dataUrl: `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`,
    width: w,
    height: h,
    srcWidth: 1200,
    srcHeight: 1620,
    format: 'png',
    printable: true,
    image: true,
    note: '',
    raw: mono,
    tearOffMm: mono ? tearMm : 0,
    tearOffPx: mono ? Math.round(tearMm * 8) : 0,
    lengthMm: mono ? Math.round(h / 8) + tearMm : 0,
  }
}

// mockDevices backs the Add-printer dialog in mock mode (browser preview).
export const mockDevices: DeviceDTO[] = [
  { uri: 'usb://Printer/POS-80?serial=936C0C663532', name: 'Printer_POS-80', info: 'Printer POS-80', makeModel: 'Printer POS-80', connection: 'USB', driver: 'raw', installed: false, queue: '' },
  { uri: 'usb://Brother/QL-820NWB?serial=000G1Z', name: 'Brother_QL-820NWB', info: 'Brother QL-820NWB', makeModel: 'Brother QL-820NWB', connection: 'USB', driver: 'raw', installed: true, queue: 'Brother QL-820' },
  { uri: 'ipp://192.168.1.42/ipp/print', name: 'HP_LaserJet_Pro', info: 'HP LaserJet Pro M428', makeModel: 'HP LaserJet Pro M428', connection: 'Wi-Fi', driver: 'everywhere', installed: true, queue: 'HP LaserJet Pro' },
  { uri: 'socket://192.168.1.77:9100', name: 'Star_TSP143', info: 'Star TSP143 (JetDirect)', makeModel: '', connection: 'Wi-Fi', driver: 'raw', installed: false, queue: '' },
]
