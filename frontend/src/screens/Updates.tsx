import { CircleCheck, Download, RefreshCw } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { agent } from '../lib/agent'
import { Button, Card, Toggle } from '../components/primitives'

const WHATS_NEW = [
  'Faster job spooling: large PDFs start printing sooner.',
  'Improved auto-reconnect for Wi-Fi and cloud printers.',
  'Live per-printer status and queue depth.',
  'Fixes a rare crash when cancelling a job mid-print.',
]

function trimV(v: string) {
  return v.startsWith('v') || v.startsWith('V') ? v.slice(1) : v
}

function GroupHeader({ children }: { children: string }) {
  return <div className="px-1 pb-2 pt-1 text-[11px] font-bold tracking-wider text-muted2">{children}</div>
}

export function UpdatesScreen({
  state,
  onInstall,
  onCheck,
}: {
  state: StateDTO
  onInstall: () => void
  onCheck: () => void
}) {
  const hasUpdate = !!state.availableVersion
  const last = state.lastCheck
    ? new Date(state.lastCheck).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    : 'never'

  return (
    <div className="h-full overflow-auto p-6">
      <div className="mx-auto flex max-w-[820px] flex-col gap-4">
        {hasUpdate ? (
          <div className="flex items-center justify-between gap-4 rounded-[13px] border border-amber/40 bg-amber-bg p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-amber/20 text-amber">
                <Download size={20} strokeWidth={2.2} />
              </div>
              <div>
                <div className="font-display text-[15px] font-bold text-amber">
                  Update available · Klutch {state.availableVersion}
                </div>
                <div className="text-[12px] text-muted">
                  You're on {trimV(state.version)} · a newer version is ready.
                </div>
              </div>
            </div>
            <Button variant="primary" icon={Download} onClick={onInstall}>
              Install &amp; restart
            </Button>
          </div>
        ) : (
          <Card className="flex items-center justify-between gap-4 p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-teal-bg text-teal">
                <CircleCheck size={20} strokeWidth={2.2} />
              </div>
              <div>
                <div className="text-[15px] font-bold text-text">You're up to date</div>
                <div className="text-[12px] text-muted">
                  Klutch {trimV(state.version)}. Last checked: {last}.
                </div>
              </div>
            </div>
            <Button variant="secondary" icon={RefreshCw} onClick={onCheck}>
              Check now
            </Button>
          </Card>
        )}

        <Card className="flex items-center justify-between p-4">
          <div>
            <div className="text-[14px] font-bold text-text">Automatic updates</div>
            <div className="text-[12px] text-muted">Download and install new versions in the background.</div>
          </div>
          <Toggle on={state.autoUpdate} onChange={(v) => agent.setAutoUpdate(v)} />
        </Card>

        <div>
          <GroupHeader>WHAT'S NEW</GroupHeader>
          <Card className="flex flex-col gap-2.5 p-4">
            {WHATS_NEW.map((t) => (
              <div key={t} className="flex items-start gap-2.5">
                <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-amber" />
                <span className="text-[13px] text-muted">{t}</span>
              </div>
            ))}
          </Card>
        </div>

        <div>
          <GroupHeader>ABOUT</GroupHeader>
          <Card className="p-4 text-[13px] text-muted">
            The agent updates itself from its release channel. Enable automatic updates above to install
            them without prompting.
          </Card>
        </div>
      </div>
    </div>
  )
}
