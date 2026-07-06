import { CircleCheck, Layers, MapPin, Plus, Printer } from 'lucide-react'
import type { PrinterDTO, StateDTO } from '../lib/types'
import { connectionIcon, statusTone } from '../lib/status'
import { agent } from '../lib/agent'
import { Button, Card, Chip, StatusPill } from '../components/primitives'

function PrinterCard({ p, onOpenQueue }: { p: PrinterDTO; onOpenQueue: () => void }) {
  const tone = statusTone(p.status)
  const ConnIcon = connectionIcon(p.connection)
  return (
    <Card className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-[38px] w-[38px] items-center justify-center rounded-[9px] bg-surface2 text-amber">
            <Printer size={17} strokeWidth={2.2} />
          </div>
          <div className="leading-tight">
            <div className="text-[15px] font-bold text-text">{p.name}</div>
            <div className="text-[12px] text-muted">{p.model || 'Printer'}</div>
          </div>
        </div>
        <StatusPill text={p.stateReason || tone.label} tone={tone} />
      </div>

      <div className="flex flex-wrap gap-2">
        <Chip icon={ConnIcon}>{p.connection}</Chip>
        {p.location && <Chip icon={MapPin}>{p.location}</Chip>}
        <Chip icon={Layers}>{p.queued} queued</Chip>
      </div>

      <div className="mt-auto grid grid-cols-2 gap-2 pt-1">
        <Button variant="secondary" icon={Layers} onClick={onOpenQueue}>
          Open queue
        </Button>
        {p.default ? (
          <Button variant="secondary" icon={CircleCheck} disabled>
            Default
          </Button>
        ) : (
          <Button variant="primary" onClick={() => agent.setDefaultPrinter(p.name)}>
            Set default
          </Button>
        )}
      </div>
    </Card>
  )
}

export function PrintersScreen({
  state,
  search,
  onAddPrinter,
  onOpenQueue,
}: {
  state: StateDTO
  search: string
  onAddPrinter: () => void
  onOpenQueue: () => void
}) {
  const q = search.trim().toLowerCase()
  const printers = state.printers.filter(
    (p) => !q || p.name.toLowerCase().includes(q) || p.model.toLowerCase().includes(q),
  )
  const online = state.printers.filter((p) => ['online', 'idle', 'printing'].includes(p.status)).length
  const attention = state.printers.filter((p) => p.status === 'error').length
  const offline = state.printers.length - online - attention

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <div>
          <div className="text-[15px] font-bold text-text">Connected printers</div>
          <div className="text-[12px] text-muted">
            {online} online · {offline} offline · {attention} need attention
          </div>
        </div>
        <Button variant="primary" icon={Plus} onClick={onAddPrinter}>
          Add printer
        </Button>
      </div>

      <div className="flex-1 overflow-auto p-6">
        {printers.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <Printer size={40} className="text-muted2" />
            <div className="text-[14px] font-bold text-text">No printers found</div>
            <div className="text-[12px] text-muted">Connect a printer, or add one from the dashboard.</div>
          </div>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
            {printers.map((p) => (
              <PrinterCard key={p.name} p={p} onOpenQueue={onOpenQueue} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
