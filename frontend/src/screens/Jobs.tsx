import { FileText, Pause, Play, RefreshCw, RotateCcw, X } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { statusTone } from '../lib/status'
import { agent } from '../lib/agent'
import { Button, ProgressBar, Segmented, StatusPill, type SegItem } from '../components/primitives'

type Row = {
  id: string
  doc: string
  sub: string
  state: 'printing' | 'queued' | 'paused' | 'done' | 'failed'
  percent: number
  reason?: string
}

function kindLabel(kind: string) {
  if (kind === 'pdf') return 'PDF'
  if (kind === 'escpos_raster') return 'Receipt'
  return 'Document'
}

function rows(state: StateDTO): Row[] {
  const seen = new Set<string>()
  const out: Row[] = []
  for (const j of state.activeJobs) {
    seen.add(j.id)
    out.push({
      id: j.id,
      doc: j.doc,
      sub: `${j.printer} · ${kindLabel(j.kind)}`,
      state: j.state as Row['state'],
      percent: j.percent,
    })
  }
  for (const r of state.recentJobs) {
    if (seen.has(r.id)) continue
    const when = new Date(r.finishedAt)
    out.push({
      id: r.id,
      doc: r.doc,
      sub: `${r.printer} · ${isNaN(+when) ? '' : when.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}`,
      state: r.status === 'failed' ? 'failed' : 'done',
      percent: 1,
      reason: r.error,
    })
  }
  return out
}

function JobRow({ r }: { r: Row }) {
  const tone = statusTone(r.state)
  const showBar = ['printing', 'queued', 'paused'].includes(r.state)
  return (
    <div className="flex items-center gap-4 px-2 py-3">
      <div className="flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[9px] bg-surface2 text-muted">
        <FileText size={16} strokeWidth={2.2} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-[14px] font-bold text-text">{r.doc}</div>
        <div className="truncate text-[11px] text-muted2">{r.reason || r.sub}</div>
      </div>
      {showBar && (
        <div className="flex w-[220px] items-center gap-3">
          <ProgressBar value={r.percent} accent={r.state === 'printing' ? 'amber' : 'muted'} />
          <span className={`w-9 shrink-0 text-right font-mono text-[11px] font-bold ${tone.text}`}>
            {r.percent >= 0 ? `${Math.round(r.percent * 100)}%` : '·'}
          </span>
        </div>
      )}
      <div className="flex shrink-0 items-center gap-2">
        <StatusPill text={tone.label} tone={tone} />
        {(r.state === 'printing' || r.state === 'queued') && (
          <Button variant="secondary" icon={Pause} onClick={() => agent.pauseJob(r.id)}>
            Pause
          </Button>
        )}
        {r.state === 'paused' && (
          <Button variant="primary" icon={Play} onClick={() => agent.resumeJob(r.id)}>
            Resume
          </Button>
        )}
        {['printing', 'queued', 'paused'].includes(r.state) && (
          <button
            type="button"
            title="Cancel job"
            onClick={() => agent.cancelJob(r.id)}
            className="flex h-[34px] w-[34px] items-center justify-center rounded-[10px] border border-border2 bg-surface2 text-muted transition hover:text-red-ink"
          >
            <X size={16} strokeWidth={2.4} />
          </button>
        )}
        {r.state === 'failed' && (
          <Button variant="danger" icon={RotateCcw} onClick={() => agent.reprintJob(r.id)}>
            Retry
          </Button>
        )}
        {r.state === 'done' && (
          <Button variant="secondary" icon={RefreshCw} onClick={() => agent.reprintJob(r.id)}>
            Reprint
          </Button>
        )}
      </div>
    </div>
  )
}

export function JobsScreen({
  state,
  search,
  filter,
  onFilter,
}: {
  state: StateDTO
  search: string
  filter: string
  onFilter: (k: string) => void
}) {
  const all = rows(state)
  const counts = {
    all: all.length,
    printing: all.filter((r) => r.state === 'printing').length,
    queued: all.filter((r) => r.state === 'queued').length,
    failed: all.filter((r) => r.state === 'failed').length,
    done: all.filter((r) => r.state === 'done').length,
  }
  const q = search.trim().toLowerCase()
  const shown = all.filter(
    (r) => (filter === 'all' || r.state === filter) && (!q || r.doc.toLowerCase().includes(q)),
  )
  const tabs: SegItem[] = [
    { key: 'all', label: 'All', count: counts.all },
    { key: 'printing', label: 'Printing', count: counts.printing },
    { key: 'queued', label: 'Queued', count: counts.queued },
    { key: 'failed', label: 'Failed', count: counts.failed },
    { key: 'done', label: 'Done', count: counts.done },
  ]

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <Segmented items={tabs} value={filter} onChange={onFilter} />
        <Button variant="secondary" icon={Pause} onClick={() => agent.pauseAll()}>
          Pause all
        </Button>
      </div>
      <div className="flex-1 overflow-auto px-4 py-2">
        {shown.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <FileText size={40} className="text-muted2" />
            <div className="text-[14px] font-bold text-text">No jobs here</div>
            <div className="text-[12px] text-muted">Print jobs will appear as they arrive.</div>
          </div>
        ) : (
          <div className="divide-y divide-border">
            {shown.map((r) => (
              <JobRow key={r.id} r={r} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
