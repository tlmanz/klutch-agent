import { useEffect, useState } from 'react'
import { Circle, CircleCheck, Loader, X } from 'lucide-react'
import { agent } from '../lib/agent'
import { Logo } from '../components/Logo'
import { Toggle } from '../components/primitives'

// The install experience (design screen 3): a checklist of the steps the agent
// performs to register itself in the OS launcher, with a progress bar. The
// backend install (desktop.Install) is a single atomic call, so the per-step
// checklist is animated over the real InstallApp() call; the steps are the
// actual things the installer does, just not individually reported.

function osName(): string {
  const ua = navigator.userAgent
  if (ua.includes('Windows')) return 'Windows'
  if (ua.includes('Mac')) return 'macOS'
  return 'Linux'
}

function copyPathLabel(): string {
  const os = osName()
  if (os === 'Windows') return 'Copy app to %LOCALAPPDATA%\\Klutch\\klutch-agent.exe'
  if (os === 'macOS') return 'Copy app to /Applications/Klutch Agent.app'
  return 'Copy app to ~/.local/bin/klutch-agent'
}

type Phase = 'ready' | 'installing' | 'done' | 'error'

export function Install({ open, onClose, version }: { open: boolean; onClose: () => void; version: string }) {
  const [phase, setPhase] = useState<Phase>('ready')
  const [step, setStep] = useState(0)
  const [err, setErr] = useState('')
  const [launch, setLaunch] = useState(true)

  useEffect(() => {
    if (open) {
      setPhase('ready')
      setStep(0)
      setErr('')
      setLaunch(true) // installing is an opt-in; default launch-at-login on
    }
  }, [open])

  // Animate the install steps while the (atomic) InstallApp call runs.
  useEffect(() => {
    if (phase !== 'installing') return
    const t = setInterval(() => setStep((s) => Math.min(s + 1, STEPS.length - 1)), 650)
    return () => clearInterval(t)
  }, [phase])

  if (!open) return null

  const labels = [
    copyPathLabel(),
    'Add application icon',
    'Create desktop entry',
    'Register in the applications menu',
    'Enable launch at login',
  ]

  const start = async () => {
    setPhase('installing')
    setStep(0)
    try {
      await agent.installApp()
      if (launch) await agent.setAutostart(true)
      setStep(STEPS.length)
      setPhase('done')
    } catch (e) {
      setErr(String(e))
      setPhase('error')
    }
  }

  const pct = phase === 'done' ? 100 : phase === 'installing' ? Math.min(90, 15 + step * 20) : 0
  const statusText =
    phase === 'done'
      ? 'Installed'
      : phase === 'error'
        ? 'Install failed'
        : `Installing… · ${labels[Math.min(step, labels.length - 1)]}`

  const primaryLabel =
    phase === 'installing' ? 'Installing…' : phase === 'done' ? 'Finish' : phase === 'error' ? 'Retry' : 'Install'
  const primaryAction = phase === 'done' ? onClose : phase === 'installing' ? undefined : start

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-bg p-10">
      <button type="button" onClick={onClose} className="absolute right-5 top-5 text-muted hover:text-text">
        <X size={20} />
      </button>
      <div className="flex w-[600px] flex-col gap-6 rounded-[20px] border border-border2 bg-base p-10">
        <div className="flex flex-col items-center gap-3.5">
          <Logo size={64} />
          <div className="font-display text-[26px] font-bold text-text">Install Klutch Agent</div>
          <div className="w-[420px] text-center font-body text-[14px] leading-5 text-muted">
            Set up Klutch to run on this device and appear in your applications.
          </div>
          <div className="rounded-full border border-border2 bg-surface px-3 py-1.5">
            <span className="font-mono text-[12px] text-muted2">
              Version {version.replace(/^v/i, '')} · {osName()}
            </span>
          </div>
        </div>

        <div className="flex flex-col gap-0.5 rounded-[14px] border border-border bg-surface p-1.5">
          {labels.map((label, i) => {
            const isToggle = i === STEPS.length - 1
            const done = phase === 'done' || step > i
            const active = phase === 'installing' && step === i
            return (
              <div
                key={label}
                className={`flex items-center gap-3 rounded-[10px] p-3 ${active ? 'bg-amber-bg' : ''}`}
              >
                {done ? (
                  <CircleCheck size={20} className="shrink-0 text-teal" />
                ) : active ? (
                  <Loader size={20} className="shrink-0 animate-spin text-amber" />
                ) : (
                  <Circle size={20} className="shrink-0 text-muted2" />
                )}
                <span
                  className={`flex-1 font-mono text-[13px] ${
                    done ? 'text-muted' : active ? 'text-amber' : 'text-text'
                  }`}
                >
                  {label}
                </span>
                {isToggle ? (
                  <Toggle on={launch} onChange={setLaunch} />
                ) : done ? (
                  <span className="font-body text-[12px] font-bold text-teal">Done</span>
                ) : null}
              </div>
            )
          })}
        </div>

        {(phase === 'installing' || phase === 'done' || phase === 'error') && (
          <div className="flex flex-col gap-2.5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                {phase === 'installing' && <Loader size={14} className="animate-spin text-amber" />}
                <span className={`font-body text-[13px] font-semibold ${phase === 'error' ? 'text-red-ink' : 'text-muted'}`}>
                  {phase === 'error' ? err || statusText : statusText}
                </span>
              </div>
              <span className="font-mono text-[13px] font-bold text-amber">{pct}%</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-surface2">
              <div className="h-full rounded-full bg-amber transition-all" style={{ width: `${pct}%` }} />
            </div>
          </div>
        )}

        <div className="flex gap-3">
          <button
            type="button"
            onClick={primaryAction}
            disabled={phase === 'installing'}
            className="flex h-12 flex-1 items-center justify-center gap-2 rounded-xl bg-amber font-body text-[15px] font-bold text-on-primary transition hover:brightness-95 disabled:opacity-60"
          >
            {phase === 'installing' && <Loader size={17} className="animate-spin" />}
            {primaryLabel}
          </button>
          <button
            type="button"
            onClick={onClose}
            className="flex h-12 items-center justify-center rounded-xl border border-border2 bg-surface px-7 font-body text-[15px] font-bold text-text2 hover:bg-surface2"
          >
            {phase === 'done' ? 'Close' : 'Cancel'}
          </button>
        </div>
      </div>
    </div>
  )
}

const STEPS = [0, 1, 2, 3, 4]
