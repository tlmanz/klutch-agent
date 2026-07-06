import { Cloud, Usb, Wifi, type LucideIcon } from 'lucide-react'

// Maps a printer/job state keyword to its pill label + Tailwind tint classes.
export interface Tone {
  label: string
  text: string // text color class
  bg: string // background tint class
  dot: string // dot fill (bg) class
}

export function statusTone(state: string): Tone {
  switch (state) {
    case 'online':
      return { label: 'Online', text: 'text-teal-ink', bg: 'bg-teal-bg', dot: 'bg-teal' }
    case 'done':
    case 'ok':
      return { label: 'Done', text: 'text-teal-ink', bg: 'bg-teal-bg', dot: 'bg-teal' }
    case 'printing':
      return { label: 'Printing', text: 'text-amber-ink', bg: 'bg-amber-bg', dot: 'bg-amber' }
    case 'queued':
      return { label: 'Queued', text: 'text-muted', bg: 'bg-surface2', dot: 'bg-muted' }
    case 'paused':
      return { label: 'Paused', text: 'text-muted', bg: 'bg-surface2', dot: 'bg-muted' }
    case 'idle':
      return { label: 'Idle', text: 'text-muted', bg: 'bg-surface2', dot: 'bg-muted' }
    case 'error':
      return { label: 'Error', text: 'text-red-ink', bg: 'bg-red-bg', dot: 'bg-red' }
    case 'failed':
      return { label: 'Failed', text: 'text-red-ink', bg: 'bg-red-bg', dot: 'bg-red' }
    default:
      return { label: 'Offline', text: 'text-muted', bg: 'bg-surface2', dot: 'bg-muted2' }
  }
}

export function connectionIcon(connection: string): LucideIcon {
  switch (connection) {
    case 'USB':
      return Usb
    case 'Cloud':
      return Cloud
    default:
      return Wifi
  }
}
