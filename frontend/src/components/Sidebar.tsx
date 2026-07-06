import { Download, Layers, Printer, Settings, type LucideIcon } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { cx } from './primitives'
import { Logo } from './Logo'
import type { ScreenId } from '../lib/screens'

const NAV: { id: ScreenId; label: string; icon: LucideIcon }[] = [
  { id: 'printers', label: 'Printers', icon: Printer },
  { id: 'jobs', label: 'Jobs', icon: Layers },
  { id: 'updates', label: 'Updates', icon: Download },
  { id: 'settings', label: 'Settings', icon: Settings },
]

function NavRow({
  item,
  active,
  count,
  onClick,
}: {
  item: (typeof NAV)[number]
  active: boolean
  count?: number
  onClick: () => void
}) {
  const Icon = item.icon
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        'flex w-full items-center gap-3 rounded-[11px] px-3 py-2.5 text-[15px] font-semibold transition',
        active ? 'bg-amber-bg text-amber' : 'text-muted hover:text-text',
      )}
    >
      <Icon size={18} strokeWidth={2.2} />
      <span className="flex-1 text-left">{item.label}</span>
      {count !== undefined && count > 0 && (
        <span className="rounded-full bg-surface2 px-1.5 py-0.5 text-[11px] font-bold text-text">
          {count}
        </span>
      )}
    </button>
  )
}

export function Sidebar({
  state,
  current,
  onNavigate,
}: {
  state: StateDTO
  current: ScreenId
  onNavigate: (id: ScreenId) => void
}) {
  const activeJobs = state.activeJobs.filter((j) =>
    ['printing', 'queued', 'paused'].includes(j.state),
  ).length
  const counts: Partial<Record<ScreenId, number>> = {
    jobs: activeJobs,
    updates: state.availableVersion ? 1 : 0,
  }
  const host = state.host || 'This device'
  return (
    <aside className="flex w-[236px] shrink-0 flex-col border-r border-border bg-sidebar px-4 py-5">
      <div className="mb-4 flex items-center gap-3 px-1">
        <Logo size={36} />
        <div className="leading-tight">
          <div className="font-display text-[16px] font-bold text-text">Klutch</div>
          <div className="text-[11px] text-muted">Print Agent</div>
        </div>
      </div>

      <nav className="flex flex-col gap-1.5">
        {NAV.map((item) => (
          <NavRow
            key={item.id}
            item={item}
            active={current === item.id}
            count={counts[item.id]}
            onClick={() => onNavigate(item.id)}
          />
        ))}
      </nav>

      <div className="flex-1" />

      <div className="border-t border-border pt-4">
        <div className="flex items-center gap-3 px-1">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-amber-bg font-bold text-amber">
            {host.slice(0, 1).toUpperCase()}
          </div>
          <div className="leading-tight">
            <div className="text-[13px] font-bold text-text">{host}</div>
            <div className="text-[11px] text-muted">This device</div>
          </div>
        </div>
      </div>
    </aside>
  )
}
