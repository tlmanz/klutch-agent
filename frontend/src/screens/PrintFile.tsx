import { useEffect, useMemo, useState } from 'react'
import {
  FileImage,
  FolderOpen,
  Info,
  Printer as PrinterIcon,
  RotateCw,
  Scissors,
} from 'lucide-react'
import type { PreviewDTO, PrintOptionsDTO, StateDTO } from '../lib/types'
import { agent } from '../lib/agent'
import { Button, Card, Segmented, Slider, Toggle } from '../components/primitives'
import { Select, TextInput } from '../components/form'

// Printing something that lives on this machine. The preview is not a mock-up:
// the agent runs the real conversion pipeline and hands back the exact image it
// will send, which for a receipt printer is the literal dot pattern at the head's
// own width. Adjusting a control re-renders through the same path.

const PAPER = [
  { key: '384', label: '58 mm' },
  { key: '576', label: '80 mm' },
]
const MEDIA = ['Printer default', 'A4', 'Letter', 'Legal', 'A5']

const baseName = (p: string) => p.split(/[\\/]/).pop() || p

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-[13px] font-bold text-text">{label}</label>
      {children}
      {hint && <div className="text-[11px] leading-relaxed text-muted">{hint}</div>}
    </div>
  )
}

export function PrintFileScreen({ state }: { state: StateDTO }) {
  const printers = state.printers
  const [path, setPath] = useState('')
  const [printer, setPrinter] = useState(state.defaultPrinter || printers[0]?.name || '')
  const [mode, setMode] = useState<'color' | 'gray' | 'mono'>('color')
  const [dither, setDither] = useState(true)
  const [threshold, setThreshold] = useState(128)
  const [rotate, setRotate] = useState(0)
  const [invert, setInvert] = useState(false)
  const [widthPx, setWidthPx] = useState(576)
  const [copies, setCopies] = useState('1')
  const [cut, setCut] = useState(true)
  const [tearOff, setTearOff] = useState(30)
  const [media, setMedia] = useState('Printer default')
  const [fitToPage, setFitToPage] = useState(true)

  const [preview, setPreview] = useState<PreviewDTO | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [sent, setSent] = useState('')

  const selected = printers.find((p) => p.name === printer)
  const raw = selected?.raw ?? false

  // A pass-through queue fires dots: colour and page scaling are not its to give,
  // so the controls follow the printer rather than offering settings it ignores.
  const effectiveMode = raw ? 'mono' : mode

  const options: PrintOptionsDTO = useMemo(
    () => ({
      path,
      printer,
      mode: effectiveMode,
      dither,
      threshold,
      rotate,
      invert,
      widthPx: raw ? widthPx : 0,
      copies: Math.min(99, Math.max(1, Number(copies) || 1)),
      cut: raw && cut,
      tearOffMm: raw ? tearOff : 0,
      align: 1,
      media: media === 'Printer default' ? '' : media,
      fitToPage: !raw && fitToPage,
    }),
    [path, printer, effectiveMode, dither, threshold, rotate, invert, raw, widthPx, copies, cut, tearOff, media, fitToPage],
  )

  // The tear-off distance belongs to the PRINTER, and is set on the Printers
  // screen. Read here so the preview draws the tear line where it will really
  // fall, and so this print uses the same feed every other receipt does.
  useEffect(() => {
    if (!printer) return
    let cancelled = false
    agent.tearOffMM(printer).then((mm) => !cancelled && setTearOff(mm))
    return () => {
      cancelled = true
    }
  }, [printer])

  // Re-render the preview as the controls move, debounced so dragging the
  // threshold slider does not queue a decode per pixel.
  useEffect(() => {
    if (!path) {
      setPreview(null)
      return
    }
    let cancelled = false
    const t = setTimeout(() => {
      agent
        .previewLocalFile(options)
        .then((p) => !cancelled && (setPreview(p), setErr('')))
        .catch((e) => !cancelled && (setPreview(null), setErr(String(e))))
    }, 180)
    return () => {
      cancelled = true
      clearTimeout(t)
    }
  }, [options, path])

  const choose = async () => {
    setErr('')
    setSent('')
    try {
      const picked = await agent.pickFile()
      if (picked) setPath(picked)
    } catch (e) {
      setErr(String(e))
    }
  }

  const print = async () => {
    setErr('')
    setSent('')
    setBusy(true)
    try {
      await agent.printLocalFile(options)
      setSent(`Sent ${baseName(path)} to ${printer}.`)
    } catch (e) {
      setErr(String(e))
    } finally {
      setBusy(false)
    }
  }

  const canPrint = Boolean(path && printer) && (preview?.printable ?? true) && !busy

  return (
    <div className="flex h-full min-h-0">
      {/* Preview */}
      <div className="flex min-w-0 flex-1 flex-col border-r border-border">
        <div className="flex items-center justify-between gap-3 border-b border-border px-6 py-4">
          <div className="min-w-0">
            <div className="truncate text-[15px] font-bold text-text">
              {path ? baseName(path) : 'No file chosen'}
            </div>
            <div className="truncate text-[12px] text-muted">
              {preview?.image
                ? `${preview.srcWidth} × ${preview.srcHeight} ${preview.format.toUpperCase()} → ${preview.width} × ${preview.height} dots`
                : path || 'Pick an image or document from this computer'}
            </div>
          </div>
          <Button variant="secondary" icon={FolderOpen} onClick={choose}>
            Choose file
          </Button>
        </div>

        <div className="flex min-h-0 flex-1 items-start justify-center overflow-auto bg-bg p-6">
          {preview?.dataUrl ? (
            <div className="flex flex-col items-center gap-3">
              <div className="w-[300px] overflow-hidden rounded-[6px] bg-white shadow-2xl">
                <img
                  src={preview.dataUrl}
                  alt="Print preview"
                  // Pixelated so a 1-bit receipt raster shows its real dots instead
                  // of being smoothed into greys the printer cannot produce.
                  style={{ imageRendering: effectiveMode === 'mono' ? 'pixelated' : 'auto' }}
                  className="block w-full"
                />
                {/* The blank paper the printer feeds so the tear bar clears the
                    print, drawn to the same scale as the image above it. */}
                {raw && preview.tearOffPx > 0 && (
                  <div
                    className="relative w-full bg-white"
                    style={{ height: `${(preview.tearOffPx / preview.width) * 300}px` }}
                  >
                    <div className="absolute inset-x-0 bottom-0 flex items-center gap-1.5 border-t border-dashed border-[#c9ccd1] px-1.5 pt-0.5">
                      <Scissors size={10} className="text-[#9aa0a6]" />
                      <span className="text-[9px] font-bold uppercase tracking-wide text-[#9aa0a6]">
                        {cut ? 'cut' : 'tear'} · {preview.tearOffMm} mm clear
                      </span>
                    </div>
                  </div>
                )}
              </div>
              {raw && (
                <div className="text-[11px] text-muted">
                  Uses about {preview.lengthMm} mm of paper
                </div>
              )}
            </div>
          ) : (
            <div className="flex max-w-[360px] flex-col items-center gap-2 text-center">
              <FileImage size={40} className="text-muted2" />
              <div className="text-[14px] font-bold text-text">
                {path ? 'No preview' : 'Nothing to preview yet'}
              </div>
              <div className="text-[12px] leading-relaxed text-muted">
                {preview?.note || err || 'Choose a PNG, JPEG or GIF to see exactly how it will print.'}
              </div>
              {!path && (
                <Button variant="primary" icon={FolderOpen} onClick={choose} className="mt-2">
                  Choose file
                </Button>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Controls */}
      <div className="flex w-[340px] shrink-0 flex-col">
        <div className="flex-1 overflow-auto p-5">
          <div className="flex flex-col gap-5">
            <Field
              label="Printer"
              hint={raw ? 'Pass-through queue: prints black & white dots at the head width.' : undefined}
            >
              <Select
                value={printer}
                options={printers.map((p) => p.name)}
                onChange={setPrinter}
                placeholder="Select a printer"
              />
            </Field>

            {raw ? (
              <>
                <Field label="Paper width" hint="Receipt heads print 384 dots across 58 mm, 576 across 80 mm.">
                  <Segmented
                    value={String(widthPx)}
                    onChange={(k) => setWidthPx(Number(k))}
                    items={PAPER}
                  />
                </Field>
                <Field label="Finish">
                  <div className="flex items-center justify-between rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5">
                    <span className="flex items-center gap-2 text-[13px] font-semibold text-text">
                      <Scissors size={14} strokeWidth={2.2} />
                      Cut paper after printing
                    </span>
                    <Toggle on={cut} onChange={setCut} />
                  </div>
                </Field>
                <Field
                  label="Tear-off feed"
                  hint="A measurement of this printer, so it lives on the Printers screen and applies to every receipt it prints - including the ones Klutch sends."
                >
                  <div className="rounded-[9px] border border-border bg-surface2 px-3 py-2 font-mono text-[12px] text-muted">
                    {tearOff} mm
                  </div>
                </Field>
              </>
            ) : (
              <>
                <Field label="Paper size">
                  <Select value={media} options={MEDIA} onChange={setMedia} />
                </Field>
                <Field label="Scaling">
                  <div className="flex items-center justify-between rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5">
                    <span className="text-[13px] font-semibold text-text">Fit to page</span>
                    <Toggle on={fitToPage} onChange={setFitToPage} />
                  </div>
                </Field>
              </>
            )}

            <Field
              label="Colour"
              hint={raw ? 'Fixed to black & white: this printer has no greys to print with.' : undefined}
            >
              <Segmented
                value={effectiveMode}
                onChange={(k) => !raw && setMode(k as typeof mode)}
                items={[
                  { key: 'color', label: 'Original' },
                  { key: 'gray', label: 'Greyscale' },
                  { key: 'mono', label: 'B & W' },
                ]}
              />
            </Field>

            {effectiveMode === 'mono' && (
              <>
                <Field
                  label="Halftone"
                  hint={
                    dither
                      ? 'Dithering fakes greys with a pattern of dots. Best for photos.'
                      : 'Every pixel is either black or white. Best for logos, text and line art.'
                  }
                >
                  <div className="flex items-center justify-between rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5">
                    <span className="text-[13px] font-semibold text-text">Dither photos</span>
                    <Toggle on={dither} onChange={setDither} />
                  </div>
                </Field>
                {!dither && (
                  <Field label="Threshold" hint="Lower keeps more white, higher turns more of the image black.">
                    <Slider value={threshold} min={16} max={240} onChange={setThreshold} label="Threshold" />
                  </Field>
                )}
              </>
            )}

            <Field label="Rotate">
              <Segmented
                value={String(rotate)}
                onChange={(k) => setRotate(Number(k))}
                items={[
                  { key: '0', label: '0°' },
                  { key: '90', label: '90°', icon: RotateCw },
                  { key: '180', label: '180°' },
                  { key: '270', label: '270°' },
                ]}
              />
            </Field>

            <Field label="Invert">
              <div className="flex items-center justify-between rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5">
                <span className="text-[13px] font-semibold text-text">Swap black and white</span>
                <Toggle on={invert} onChange={setInvert} />
              </div>
            </Field>

            <Field label="Copies">
              <TextInput value={copies} onChange={setCopies} className="w-[110px]" />
            </Field>

            {preview?.note && (
              <Card className="flex gap-2 p-3">
                <Info size={14} className="mt-0.5 shrink-0 text-muted" />
                <span className="text-[11px] leading-relaxed text-muted">{preview.note}</span>
              </Card>
            )}
          </div>
        </div>

        <div className="border-t border-border p-5">
          {err && <div className="mb-2 text-[12px] font-semibold text-red-ink">{err}</div>}
          {sent && <div className="mb-2 text-[12px] font-semibold text-teal-ink">{sent}</div>}
          <Button variant="primary" icon={PrinterIcon} full disabled={!canPrint} onClick={print}>
            {busy ? 'Sending…' : 'Print'}
          </Button>
        </div>
      </div>
    </div>
  )
}
