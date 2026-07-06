import { useEffect, useState, type ReactNode } from 'react'
import { LogIn, Moon, Printer, RefreshCw, Sun, Trash2, Download } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { agent } from '../lib/agent'
import { Button, Card, Segmented, Toggle } from '../components/primitives'
import { Select, TextInput } from '../components/form'

function trimV(v: string) {
  return v.startsWith('v') || v.startsWith('V') ? v.slice(1) : v
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <div className="px-1 pb-2 pt-1 text-[11px] font-bold tracking-wider text-muted2">{title}</div>
      <Card className="divide-y divide-border">{children}</Card>
    </div>
  )
}

function Row({ title, desc, children }: { title: string; desc: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 p-4">
      <div>
        <div className="text-[14px] font-bold text-text">{title}</div>
        <div className="text-[12px] text-muted">{desc}</div>
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}

export function SettingsScreen({
  state,
  onCheck,
  onInstallApp,
  onReconnect,
}: {
  state: StateDTO
  onCheck: () => void
  onInstallApp: () => void
  onReconnect: () => void
}) {
  const [autostart, setAutostart] = useState(false)
  const [installed, setInstalled] = useState(false)
  const [server, setServer] = useState(state.server)
  const [savedServer, setSavedServer] = useState(state.server)

  useEffect(() => {
    agent.autostartEnabled().then(setAutostart)
    agent.isAppInstalled().then(setInstalled)
  }, [])

  const printerNames = state.printers.map((p) => p.name)

  return (
    <div className="h-full overflow-auto p-6">
      <div className="mx-auto flex max-w-[820px] flex-col gap-4">
        <Group title="CONNECTION">
          <Row title="Backend server" desc="Where this agent sends heartbeats and receives jobs.">
            <div className="flex items-center gap-2">
              <TextInput value={server} onChange={setServer} className="w-[240px]" />
              <Button
                variant="secondary"
                disabled={!server || server === savedServer}
                onClick={() => {
                  agent.setServer(server)
                  setSavedServer(server)
                }}
              >
                Save
              </Button>
            </div>
          </Row>
          <Row title="Enrollment" desc="Reconnect or pair this device again.">
            <Button variant="primary" icon={LogIn} onClick={onReconnect}>
              Reconnect
            </Button>
          </Row>
        </Group>

        <Group title="GENERAL">
          <Row title="Launch at login" desc="Start Klutch automatically when this device boots.">
            <Toggle
              on={autostart}
              onChange={(v) => {
                setAutostart(v)
                agent.setAutostart(v)
              }}
            />
          </Row>
          <Row title="Default printer" desc="Used where a job doesn't pin one.">
            <Select
              className="w-[220px]"
              value={state.defaultPrinter}
              options={printerNames}
              onChange={(v) => agent.setDefaultPrinter(v)}
            />
          </Row>
        </Group>

        <Group title="NOTIFICATIONS">
          <Row title="Job completed" desc="Notify when a print finishes.">
            <Toggle on={state.notifyDone} onChange={(v) => agent.setNotifyDone(v)} />
          </Row>
          <Row title="Job failed" desc="Alert on errors, jams and out-of-paper.">
            <Toggle on={state.notifyFailed} onChange={(v) => agent.setNotifyFailed(v)} />
          </Row>
          <Row title="Weekly summary" desc="A digest of what you printed each week.">
            <Toggle on={state.notifyWeekly} onChange={(v) => agent.setNotifyWeekly(v)} />
          </Row>
        </Group>

        <Group title="APPEARANCE">
          <Row title="Theme" desc="Match the dashboard, or go light.">
            <Segmented
              value={state.theme === 'light' ? 'light' : 'dark'}
              onChange={(k) => agent.setTheme(k)}
              items={[
                { key: 'dark', label: 'Dark', icon: Sun },
                { key: 'light', label: 'Light', icon: Moon },
              ]}
            />
          </Row>
        </Group>

        <Group title="ABOUT">
          <div className="flex items-center justify-between gap-4 p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-surface2 text-amber">
                <Printer size={18} strokeWidth={2.2} />
              </div>
              <div>
                <div className="text-[14px] font-bold text-text">Klutch Print Agent</div>
                <div className="text-[12px] text-muted">Version {trimV(state.version)}</div>
              </div>
            </div>
            <Button variant="secondary" icon={RefreshCw} onClick={onCheck}>
              Check for updates
            </Button>
          </div>
          <div className="flex items-center justify-between gap-4 p-4">
            <div className="text-[12px] text-muted">
              {installed
                ? 'Installed. Find "Klutch Agent" in your applications menu.'
                : 'Add Klutch to your applications so you can reopen it after quitting.'}
            </div>
            <div className="flex shrink-0 gap-2">
              {installed ? (
                <Button
                  variant="secondary"
                  icon={Trash2}
                  onClick={() => agent.uninstallApp().then(() => setInstalled(false))}
                >
                  Remove
                </Button>
              ) : (
                <Button variant="primary" icon={Download} onClick={onInstallApp}>
                  Add to Applications
                </Button>
              )}
            </div>
          </div>
        </Group>
      </div>
    </div>
  )
}
