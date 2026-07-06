import { useEffect, useState } from 'react'
import { Sidebar } from './components/Sidebar'
import { Header } from './components/Header'
import { PrintersScreen } from './screens/Printers'
import { JobsScreen } from './screens/Jobs'
import { UpdatesScreen } from './screens/Updates'
import { SettingsScreen } from './screens/Settings'
import { Onboarding } from './screens/Onboarding'
import { Install } from './screens/Install'
import { EnrollModal } from './components/EnrollModal'
import { CompactPanel } from './components/CompactPanel'
import { ConfirmModal } from './components/form'
import { useAgentState } from './lib/useAgentState'
import { agent } from './lib/agent'
import { onEvent, windowSetSize } from './lib/runtime'
import { SCREEN_TITLES, type ScreenId } from './lib/screens'

// The URL hash seeds the initial view (used by the screenshot harness, harmless
// in the webview): #jobs, #onboarding, #install, #compact, …
const hash = () => window.location.hash.replace('#', '')

function hashScreen(): ScreenId {
  const h = hash() as ScreenId
  return (['printers', 'jobs', 'updates', 'settings'] as ScreenId[]).includes(h) ? h : 'printers'
}

export default function App() {
  const state = useAgentState()
  const [screen, setScreen] = useState<ScreenId>(hashScreen())
  const [search, setSearch] = useState('')
  const [jobFilter, setJobFilter] = useState('all')
  const [enrollOpen, setEnrollOpen] = useState(false)
  const [updateOpen, setUpdateOpen] = useState(false)
  const [installOpen, setInstallOpen] = useState(hash() === 'install')
  const [installChecked, setInstallChecked] = useState(false)
  const [compact, setCompact] = useState(hash() === 'compact')

  useEffect(() => {
    if (!state) return
    document.documentElement.classList.toggle('light', state.theme === 'light')
  }, [state?.theme])

  useEffect(() => onEvent('mode', (m) => enterMode(m === 'compact')), [])

  // Desktop notifications for terminal jobs (Go emits gated by prefs).
  useEffect(() => {
    if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
      Notification.requestPermission().catch(() => {})
    }
    return onEvent('notify', (p) => {
      const n = p as { kind: string; doc: string; printer: string; error: string }
      if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
      if (n.kind === 'done') new Notification('Print finished', { body: `${n.doc} printed on ${n.printer}` })
      else new Notification(`Print failed: ${n.doc}`, { body: n.error || 'The job failed to print.' })
    })
  }, [])

  // Once enrolled, offer the install screen a single time if not yet installed.
  useEffect(() => {
    if (!state?.enrolled || installChecked) return
    setInstallChecked(true)
    agent.isAppInstalled().then((inst) => {
      if (!inst) setInstallOpen(true)
    })
  }, [state?.enrolled, installChecked])

  const enterMode = (toCompact: boolean) => {
    setCompact(toCompact)
    windowSetSize(toCompact ? 412 : 1080, toCompact ? 600 : 720)
  }

  if (!state) {
    return <div className="flex h-full items-center justify-center text-muted">Starting…</div>
  }

  // First-run onboarding takes over the whole window until connected.
  if ((!state.enrolled || hash() === 'onboarding') && hash() !== 'install') {
    return <Onboarding state={state} />
  }

  if (compact) {
    return <CompactPanel state={state} onExpand={() => enterMode(false)} />
  }

  const toggleTheme = () => agent.setTheme(state.theme === 'light' ? 'dark' : 'light')

  return (
    <div className="flex h-full overflow-hidden bg-bg text-text">
      <Sidebar state={state} current={screen} onNavigate={setScreen} />
      <div className="flex min-w-0 flex-1 flex-col">
        <Header
          title={SCREEN_TITLES[screen]}
          state={state}
          search={search}
          onSearch={setSearch}
          onToggleTheme={toggleTheme}
        />
        <main className="min-h-0 flex-1">
          {screen === 'printers' && (
            <PrintersScreen
              state={state}
              search={search}
              onAddPrinter={() => setEnrollOpen(true)}
              onOpenQueue={() => setScreen('jobs')}
            />
          )}
          {screen === 'jobs' && (
            <JobsScreen state={state} search={search} filter={jobFilter} onFilter={setJobFilter} />
          )}
          {screen === 'updates' && (
            <UpdatesScreen state={state} onInstall={() => setUpdateOpen(true)} onCheck={() => agent.checkForUpdate()} />
          )}
          {screen === 'settings' && (
            <SettingsScreen
              state={state}
              onCheck={() => agent.checkForUpdate()}
              onInstallApp={() => setInstallOpen(true)}
              onReconnect={() => setEnrollOpen(true)}
            />
          )}
        </main>
      </div>

      <EnrollModal open={enrollOpen} onClose={() => setEnrollOpen(false)} defaultServer={state.server} />
      <Install open={installOpen} onClose={() => setInstallOpen(false)} version={state.version} />
      <ConfirmModal
        open={updateOpen}
        onClose={() => setUpdateOpen(false)}
        onConfirm={() => agent.applyUpdate()}
        title="Install update"
        body="The agent will update and restart. Continue?"
        confirmLabel="Install & restart"
      />
    </div>
  )
}
