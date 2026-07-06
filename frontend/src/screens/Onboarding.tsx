import { useState } from 'react'
import { Activity, Download, FileKey, Globe, Hash, Info, KeyRound, PlugZap, Printer, type LucideIcon } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { agent } from '../lib/agent'
import { Logo } from '../components/Logo'

const FEATURES: { icon: LucideIcon; title: string; desc: string }[] = [
  { icon: Printer, title: 'Every printer, one place', desc: 'Discover and manage local & network queues.' },
  { icon: Activity, title: 'Live job tracking', desc: 'Watch spooling and status in real time.' },
  { icon: Download, title: 'Automatic updates', desc: 'Stay current with background updates.' },
]

// The first-run onboarding: a full two-pane experience (brand story on the left,
// enrollment card on the right) shown until the agent is connected. Replaces the
// old first-run enroll modal.
export function Onboarding({ state }: { state: StateDTO }) {
  const [server, setServer] = useState(state.server)
  const [mode, setMode] = useState<'code' | 'token'>('code')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const connect = async () => {
    setErr('')
    if (mode === 'token' && !token.trim()) return setErr('Enter a device token.')
    if (mode === 'code' && !code.trim()) return setErr('Enter a pairing code.')
    setBusy(true)
    try {
      if (server && server !== state.server) await agent.setServer(server)
      if (mode === 'token') await agent.setToken(token.trim())
      else await agent.enroll(code.trim())
      // On success the state event flips `enrolled`, and App swaps to the dashboard.
    } catch (e) {
      setErr(String(e))
    } finally {
      setBusy(false)
    }
  }

  const foot = busy
    ? { dot: 'bg-amber', text: 'Connecting…', cls: 'text-amber' }
    : err
      ? { dot: 'bg-red', text: err, cls: 'text-red-ink' }
      : { dot: 'bg-muted2', text: 'Not connected yet', cls: 'text-muted2' }

  return (
    <div className="flex h-full bg-bg">
      {/* Brand pane */}
      <div className="flex w-[452px] shrink-0 flex-col justify-between bg-sidebar p-[52px_44px]">
        <div className="flex flex-col gap-[34px]">
          <div className="flex items-center gap-3">
            <Logo size={38} />
            <div className="leading-tight">
              <div className="font-display text-[18px] font-bold tracking-[0.5px] text-text">KLUTCH</div>
              <div className="font-body text-[11px] text-muted2">v{trimV(state.version)} · agent</div>
            </div>
          </div>
          <div className="flex flex-col gap-3.5">
            <span className="w-fit rounded-full bg-amber-bg px-3 py-1.5 font-body text-[11px] font-bold tracking-[1px] text-amber">
              SETUP · STEP 1 OF 1
            </span>
            <h1 className="font-display text-[32px] font-bold leading-[37px] text-text">
              Connect this device to your workspace.
            </h1>
            <p className="font-body text-[14px] leading-[21px] text-muted">
              Pair the Klutch agent with your dashboard to start managing printers and jobs across
              this machine.
            </p>
          </div>
        </div>

        <div className="flex flex-col gap-4">
          {FEATURES.map((f) => (
            <div key={f.title} className="flex items-center gap-3">
              <div className="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-[11px] border border-border2 bg-surface2 text-amber">
                <f.icon size={17} strokeWidth={2.2} />
              </div>
              <div className="leading-tight">
                <div className="font-body text-[14px] font-bold text-text">{f.title}</div>
                <div className="font-body text-[12px] leading-[17px] text-muted2">{f.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="w-px shrink-0 bg-border" />

      {/* Setup card */}
      <div className="flex flex-1 items-center justify-center p-[40px_64px]">
        <div className="flex w-[468px] flex-col gap-5 rounded-[20px] border border-border bg-base p-8">
          <div className="flex flex-col gap-1.5">
            <div className="font-body text-[12px] font-bold tracking-[1px] text-muted2">SET UP THIS AGENT</div>
            <div className="font-display text-[22px] font-bold text-text">Enroll device</div>
          </div>

          <Field label="Backend server">
            <div className="flex items-center gap-2.5 rounded-[11px] border border-border2 bg-surface2 px-3.5 py-3 focus-within:border-amber/60">
              <Globe size={15} className="shrink-0 text-muted2" />
              <input
                value={server}
                onChange={(e) => setServer(e.target.value)}
                placeholder="https://api.example.com"
                className="w-full bg-transparent font-mono text-[13px] text-text placeholder:text-muted2 focus:outline-none"
              />
            </div>
          </Field>

          <Field label="Connect using">
            <div className="flex gap-1 rounded-[11px] border border-border2 bg-surface p-1">
              <SegBtn active={mode === 'code'} icon={KeyRound} label="Pairing code" onClick={() => setMode('code')} />
              <SegBtn active={mode === 'token'} icon={FileKey} label="Device token" onClick={() => setMode('token')} />
            </div>
          </Field>

          {mode === 'code' ? (
            <Field label="Pairing code">
              <div className="flex items-center gap-2.5 rounded-[11px] border border-border2 bg-surface2 px-3.5 py-3 focus-within:border-amber/60">
                <Hash size={15} className="shrink-0 text-muted2" />
                <input
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="one-time code from the dashboard"
                  className="w-full bg-transparent text-[13px] text-text placeholder:text-muted2 focus:outline-none"
                  autoFocus
                />
              </div>
              <div className="flex items-center gap-1.5 text-muted2">
                <Info size={13} />
                <span className="font-body text-[12px]">Find your code in Dashboard → Agents → Add device.</span>
              </div>
            </Field>
          ) : (
            <Field label="Device token">
              <textarea
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="paste the device token"
                rows={3}
                className="w-full resize-none rounded-[11px] border border-border2 bg-surface2 px-3.5 py-3 text-[13px] text-text placeholder:text-muted2 focus:border-amber/60 focus:outline-none"
                autoFocus
              />
            </Field>
          )}

          <div className="h-px w-full bg-border" />

          <div className="flex flex-col gap-3">
            <button
              type="button"
              onClick={connect}
              disabled={busy}
              className="flex w-full items-center justify-center gap-2 rounded-xl bg-amber py-3.5 font-display text-[14px] font-bold text-on-primary transition hover:brightness-95 disabled:opacity-50"
            >
              <PlugZap size={16} strokeWidth={2.4} />
              {busy ? 'Connecting…' : 'Connect agent'}
            </button>
            <div className="flex items-center justify-center gap-2.5">
              <span className={`h-[7px] w-[7px] rounded-full ${foot.dot}`} />
              <span className={`font-body text-[12px] font-semibold ${foot.cls}`}>{foot.text}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <div className="font-body text-[13px] font-bold text-text2">{label}</div>
      {children}
    </div>
  )
}

function SegBtn({ active, icon: Icon, label, onClick }: { active: boolean; icon: LucideIcon; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex flex-1 items-center justify-center gap-1.5 rounded-lg py-2.5 font-body text-[13px] font-bold transition ${
        active ? 'bg-amber text-on-primary' : 'text-muted hover:text-text'
      }`}
    >
      <Icon size={15} strokeWidth={2.2} />
      {label}
    </button>
  )
}

function trimV(v: string) {
  return v.startsWith('v') || v.startsWith('V') ? v.slice(1) : v
}
