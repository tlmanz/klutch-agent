import { Pause, Printer } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { statusTone } from '../lib/status'
import { agent } from '../lib/agent'
import { Button, ProgressBar, StatusDot } from './primitives'
import { Logo } from './Logo'

// The compact tray panel (the "1b" design). In Wails v2 (single-window) this is a
// resized window mode rather than a separate window; "Open Klutch" returns to the
// full layout.
export function CompactPanel({ state, onExpand }: { state: StateDTO; onExpand: () => void }) {
  const printing = state.activeJobs.find((j) => j.state === 'printing')
  const conn = !state.enrolled
    ? { text: 'Not set up', dot: 'bg-amber' }
    : state.connected
      ? { text: 'All printers reachable', dot: 'bg-teal' }
      : { text: 'Disconnected', dot: 'bg-red' }

  return (
    <div className="flex h-full flex-col bg-base">
      <div className="flex items-center gap-3 border-b border-border px-4 py-3.5">
        <Logo size={32} />
        <div className="flex-1 leading-tight">
          <div className="font-display text-[16px] font-bold text-text">Klutch</div>
          <div className="flex items-center gap-1.5 text-[11px] font-semibold text-muted">
            <span className={`h-[7px] w-[7px] rounded-full ${conn.dot}`} />
            {conn.text}
          </div>
        </div>
      </div>

      {printing && (
        <div className="mx-3.5 mt-3 rounded-[13px] bg-amber-bg p-3.5">
          <div className="mb-1.5 flex items-center gap-2 text-[12px] font-bold text-amber-ink">
            <span className="h-2 w-2 rounded-full bg-amber" />
            Now printing · {printing.printer}
          </div>
          <div className="mb-2 text-[15px] font-bold text-text">{printing.doc}</div>
          <ProgressBar value={printing.percent} />
          <div className="mt-1.5 font-mono text-[11px] font-bold text-amber-ink">
            {printing.percent >= 0 ? `${Math.round(printing.percent * 100)}%` : '·'}
          </div>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-auto px-3.5 py-2">
        {state.printers.map((p, i) => {
          const tone = statusTone(p.status)
          return (
            <div
              key={p.name}
              className={`flex items-center gap-3 py-3 ${i > 0 ? 'border-t border-border' : ''}`}
            >
              <div className="flex h-9 w-9 items-center justify-center rounded-[9px] bg-surface2 text-muted">
                <Printer size={17} strokeWidth={2.2} />
              </div>
              <div className="min-w-0 flex-1 leading-tight">
                <div className="truncate text-[14px] font-bold text-text">{p.name}</div>
                <div className="truncate text-[11px] text-muted2">
                  {p.connection} · {p.queued} queued
                </div>
              </div>
              <StatusDot text={tone.label} tone={tone} />
            </div>
          )
        })}
      </div>

      <div className="grid grid-cols-2 gap-2.5 border-t border-border p-3.5">
        <Button variant="secondary" icon={Pause} full onClick={() => agent.pauseAll()}>
          Pause all
        </Button>
        <Button variant="primary" full onClick={onExpand}>
          Open Klutch
        </Button>
      </div>
    </div>
  )
}
