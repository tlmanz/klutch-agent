import { useEffect, useState } from 'react'
import { CircleCheck, Layers, MapPin, Plus, Printer, Scissors, Trash2 } from 'lucide-react'
import type { PrinterDTO, StateDTO } from '../lib/types'
import { connectionIcon, statusTone } from '../lib/status'
import { agent } from '../lib/agent'
import { Button, Card, Chip, Slider, StatusPill, Toggle } from '../components/primitives'
import { ConfirmModal, Modal } from '../components/form'

// Mirrors MaxTearOffMM in internal/agent: a guard against a mistyped number, not
// a hardware limit. The agent clamps to the same value, so the field never
// promises a feed the printer will not be given.
const MAX_TEAR_OFF_MM = 100

// PaperSettingsModal is how a receipt printer FINISHES a job: how much blank paper
// to feed, and whether to cut. Both live on the printer because both are facts
// about this chassis that nothing reports - the gap between the print head and the
// tear bar, and whether there is a cutter at all - so they can only come from
// whoever is looking at the machine. Every receipt it prints uses them: the ones
// Klutch sends and the files printed from this app alike.
//
// A dialog rather than an inline block on the card: these need a sentence of
// explanation each to be set correctly, and that much prose repeated down a grid
// of cards buries the thing the grid is for - seeing at a glance which printers
// are working.
function PaperSettingsModal({ printer, open, onClose }: { printer: string; open: boolean; onClose: () => void }) {
  const [mm, setMM] = useState(30)
  // Held as text so the field can be emptied mid-typing instead of snapping to 0.
  const [typed, setTyped] = useState('30')
  const [cut, setCut] = useState(false)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    agent.tearOffMM(printer).then((v) => {
      if (cancelled) return
      setMM(v)
      setTyped(String(v))
    })
    agent.cutEnabled(printer).then((v) => !cancelled && setCut(v))
    return () => {
      cancelled = true
    }
  }, [printer, open])

  // Saved once the value settles, not on every pixel of a drag or every keystroke.
  useEffect(() => {
    if (!open) return
    const t = setTimeout(() => agent.setTearOffMM(printer, mm).catch(() => {}), 400)
    return () => clearTimeout(t)
  }, [printer, open, mm])

  // Digits only, and a cleared field is left alone until blur so it can be
  // retyped rather than jumping to 0 under the cursor.
  const type = (text: string) => {
    const digits = text.replace(/[^0-9]/g, '')
    setTyped(digits)
    if (digits === '') return
    setMM(Math.min(MAX_TEAR_OFF_MM, Number(digits)))
  }
  const slide = (v: number) => {
    setMM(v)
    setTyped(String(v))
  }

  return (
    <Modal open={open} onClose={onClose} title="Paper settings" width={420}
      footer={<Button variant="primary" onClick={onClose}>Done</Button>}>
      <div className="flex flex-col gap-5">
        <div className="text-[12px] text-muted">{printer}</div>

        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-3">
            <label htmlFor="tear-off" className="text-[14px] font-bold text-text">
              Tear-off feed
            </label>
            {/* A text field with a numeric keypad rather than type="number": the
                browser's spinner arrows crowd a field this small and add a second
                way to change a value the slider below already handles. */}
            <div className="flex shrink-0 items-center gap-1.5 rounded-[9px] border border-border2 bg-surface px-3 py-1.5 focus-within:border-amber/60">
              <input
                id="tear-off"
                type="text"
                inputMode="numeric"
                maxLength={3}
                value={typed}
                onChange={(e) => type(e.target.value)}
                onBlur={() => setTyped(String(mm))}
                className="w-9 bg-transparent text-right font-mono text-[13px] text-text focus:outline-none"
              />
              <span className="text-[12px] font-bold text-muted">mm</span>
            </div>
          </div>
          <Slider
            value={mm}
            min={0}
            max={MAX_TEAR_OFF_MM}
            onChange={slide}
            label="Tear-off feed in millimetres"
            showValue={false}
          />
          <div className="text-[12px] leading-relaxed text-muted">
            Blank paper fed after every receipt, so the print clears the tear bar before you pull it. If tearing cuts
            into the bottom of a receipt, add that much here.
          </div>
        </div>

        <div className="flex items-start justify-between gap-4 border-t border-border pt-4">
          <div className="min-w-0">
            <div className="text-[14px] font-bold text-text">Cut the paper</div>
            <div className="text-[12px] leading-relaxed text-muted">
              Only if this printer has a cutter. One without a cutter ignores the command - the feed above is what
              makes tearing by hand safe.
            </div>
          </div>
          <Toggle
            on={cut}
            onChange={(v) => {
              setCut(v)
              agent.setCutEnabled(printer, v).catch(() => {})
            }}
          />
        </div>
      </div>
    </Modal>
  )
}

function PrinterCard({
  p,
  onOpenQueue,
  onRemove,
  onPaperSettings,
}: {
  p: PrinterDTO
  onOpenQueue: () => void
  onRemove: () => void
  onPaperSettings: () => void
}) {
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

      <div
        className={`mt-auto grid gap-2 pt-1 ${p.raw ? 'grid-cols-[1fr_1fr_auto_auto]' : 'grid-cols-[1fr_1fr_auto]'}`}
      >
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
        {/* Receipt queues only: a page printer ejects the sheet itself, so it has
            no tear-off feed and no cutter to ask about - and no column here. */}
        {p.raw && (
          <Button variant="secondary" icon={Scissors} title={`Paper settings for ${p.name}`} onClick={onPaperSettings} />
        )}
        <Button variant="danger" icon={Trash2} title={`Remove ${p.name}`} onClick={onRemove} />
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
  const [removing, setRemoving] = useState<PrinterDTO | null>(null)
  const [paperFor, setPaperFor] = useState<PrinterDTO | null>(null)
  const [err, setErr] = useState('')
  const q = search.trim().toLowerCase()
  // The placeholder row is a stand-in for "this PC has no printers", not a queue.
  // It has no business on the screen whose whole job is managing queues: every
  // action here talks to the spooler, and there is nothing behind it to act on -
  // Remove used to hand its invented name to lpadmin, which rejected it. Dropped,
  // the list is genuinely empty and the empty state below says the useful thing.
  const real = state.printers.filter((p) => !p.placeholder)
  const printers = real.filter((p) => !q || p.name.toLowerCase().includes(q) || p.model.toLowerCase().includes(q))
  const online = real.filter((p) => ['online', 'idle', 'printing'].includes(p.status)).length
  const attention = real.filter((p) => p.status === 'error').length
  const offline = real.length - online - attention

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
        {err && (
          <div className="mb-4 rounded-[11px] border border-red-bg bg-red-bg/40 px-4 py-2.5 text-[13px] font-semibold text-red-ink">
            {err}
          </div>
        )}
        {printers.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <Printer size={40} className="text-muted2" />
            <div className="text-[14px] font-bold text-text">No printers found</div>
            <div className="text-[12px] text-muted">
              Plug in a printer, then use Add printer to set up its queue.
            </div>
            <Button variant="primary" icon={Plus} onClick={onAddPrinter} className="mt-2">
              Add printer
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
            {printers.map((p) => (
              <PrinterCard
                key={p.name}
                p={p}
                onOpenQueue={onOpenQueue}
                onPaperSettings={() => setPaperFor(p)}
                onRemove={() => {
                  setErr('')
                  setRemoving(p)
                }}
              />
            ))}
          </div>
        )}
      </div>

      <PaperSettingsModal
        printer={paperFor?.name ?? ''}
        open={paperFor !== null}
        onClose={() => setPaperFor(null)}
      />

      <ConfirmModal
        open={removing !== null}
        onClose={() => setRemoving(null)}
        onConfirm={() => {
          const name = removing?.name
          if (name) agent.removePrinter(name).catch((e) => setErr(String(e)))
        }}
        title="Remove printer"
        body={`"${removing?.name}" will be deleted from this computer's print system. Queued jobs for it are lost. You can add it again later.`}
        confirmLabel="Remove"
        danger
      />
    </div>
  )
}
