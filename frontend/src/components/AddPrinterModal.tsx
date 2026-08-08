import { useCallback, useEffect, useState } from 'react'
import { Check, Plug, RefreshCw } from 'lucide-react'
import type { DeviceDTO } from '../lib/types'
import { connectionIcon } from '../lib/status'
import { agent } from '../lib/agent'
import { Button, cx } from './primitives'
import { Modal, Select, TextInput } from './form'

// Adding a printer the spooler did not set up itself (the common case for a USB
// receipt printer): scan the OS for reachable devices, name the queue, pick how
// the bytes should be sent, create it. Anything already attached to a queue is
// listed too, marked as added, so it is obvious the scan saw it.

// Driver choices. Raw is the default because the backend sends already-rendered
// bytes (PDF / ESC-POS raster) that a driver must not re-process.
const DRIVERS = [
  { key: 'raw', label: 'Raw (pass-through)' },
  { key: 'everywhere', label: 'IPP Everywhere (driverless)' },
]
const driverLabel = (key: string) => DRIVERS.find((d) => d.key === key)?.label ?? DRIVERS[0].label
const driverKey = (label: string) => DRIVERS.find((d) => d.label === label)?.key ?? 'raw'

function DeviceRow({
  device,
  selected,
  onSelect,
}: {
  device: DeviceDTO
  selected: boolean
  onSelect: () => void
}) {
  const Icon = connectionIcon(device.connection)
  return (
    <button
      type="button"
      disabled={device.installed}
      onClick={onSelect}
      className={cx(
        'flex w-full items-center gap-3 rounded-[11px] border px-3 py-2.5 text-left transition',
        'disabled:cursor-not-allowed disabled:opacity-55',
        selected ? 'border-amber bg-amber-bg/40' : 'border-border2 bg-surface hover:bg-surface2',
      )}
    >
      <Icon size={16} className={cx('shrink-0', selected ? 'text-amber' : 'text-muted')} strokeWidth={2.2} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[13px] font-bold text-text">
          {device.info || device.makeModel || device.uri}
        </div>
        <div className="truncate text-[11px] text-muted">{device.uri}</div>
      </div>
      {device.installed ? (
        <span className="shrink-0 text-[11px] font-bold text-muted">Added as {device.queue}</span>
      ) : (
        selected && <Check size={15} className="shrink-0 text-amber" />
      )}
    </button>
  )
}

export function AddPrinterModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [devices, setDevices] = useState<DeviceDTO[]>([])
  const [scanning, setScanning] = useState(false)
  const [uri, setUri] = useState('')
  const [name, setName] = useState('')
  const [driver, setDriver] = useState('raw')
  const [manual, setManual] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const scan = useCallback(async () => {
    setScanning(true)
    setErr('')
    try {
      setDevices(await agent.discoverDevices())
    } catch (e) {
      setErr(String(e))
    } finally {
      setScanning(false)
    }
  }, [])

  // Rescan on every open: a printer plugged in while the dialog was shut should
  // be there when it reopens.
  useEffect(() => {
    if (!open) return
    setUri('')
    setName('')
    setDriver('raw')
    setManual(false)
    setErr('')
    scan()
  }, [open, scan])

  const select = (d: DeviceDTO) => {
    setUri(d.uri)
    setName(d.name)
    setDriver(d.driver || 'raw')
    setErr('')
  }

  const add = async () => {
    setErr('')
    if (!uri.trim()) return setErr('Select a printer, or enter its address.')
    if (!name.trim()) return setErr('Enter a name for the printer.')
    setBusy(true)
    try {
      await agent.addPrinter(name.trim(), uri.trim(), driver)
      onClose()
    } catch (e) {
      setErr(String(e))
    } finally {
      setBusy(false)
    }
  }

  const available = devices.filter((d) => !d.installed)

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Add a printer"
      width={520}
      footer={
        <>
          <Button variant="secondary" icon={RefreshCw} onClick={scan} disabled={scanning || busy}>
            {scanning ? 'Scanning…' : 'Rescan'}
          </Button>
          <Button variant="primary" onClick={add} disabled={busy || scanning}>
            {busy ? 'Adding…' : 'Add printer'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <div className="mb-1.5 flex items-center justify-between">
            <label className="text-[13px] font-bold text-text">Detected devices</label>
            <span className="text-[11px] text-muted">
              {scanning ? 'Scanning…' : `${available.length} available`}
            </span>
          </div>
          <div className="flex max-h-[210px] flex-col gap-1.5 overflow-auto">
            {devices.map((d) => (
              <DeviceRow key={d.uri} device={d} selected={d.uri === uri} onSelect={() => select(d)} />
            ))}
            {!scanning && devices.length === 0 && (
              <div className="flex flex-col items-center gap-1.5 rounded-[11px] border border-dashed border-border2 py-6 text-center">
                <Plug size={20} className="text-muted2" />
                <div className="text-[12px] text-muted">
                  No devices found. Check the cable and power, then rescan.
                </div>
              </div>
            )}
          </div>
        </div>

        <button
          type="button"
          onClick={() => {
            setManual((v) => !v)
            setUri('')
          }}
          className="self-start text-[12px] font-bold text-amber hover:underline"
        >
          {manual ? 'Choose a detected device instead' : 'Not listed? Enter a network address'}
        </button>

        {manual && (
          <div>
            <label className="mb-1.5 block text-[13px] font-bold text-text">Device address</label>
            <TextInput value={uri} onChange={setUri} placeholder="socket://192.168.1.50:9100" />
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="mb-1.5 block text-[13px] font-bold text-text">Printer name</label>
            <TextInput value={name} onChange={setName} placeholder="Front_Desk_Receipt" />
          </div>
          <div>
            <label className="mb-1.5 block text-[13px] font-bold text-text">Send data as</label>
            <Select
              value={driverLabel(driver)}
              options={DRIVERS.map((d) => d.label)}
              onChange={(label) => setDriver(driverKey(label))}
            />
          </div>
        </div>
        <p className="text-[11px] leading-relaxed text-muted">
          Receipt and label printers (ESC/POS, ZPL) need <span className="font-bold">Raw</span>; the
          server already renders what they print. Use IPP Everywhere for modern network printers.
        </p>

        {err && <div className="text-[13px] font-semibold text-red-ink">{err}</div>}
      </div>
    </Modal>
  )
}
