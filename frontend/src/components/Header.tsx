import { AlertTriangle, CircleCheck, LogIn, Moon, Sun } from 'lucide-react'
import type { StateDTO } from '../lib/types'
import { IconButton, cx } from './primitives'
import { SearchInput } from './form'

function ConnPill({ state }: { state: StateDTO }) {
  let Icon = CircleCheck
  let text = 'Connected'
  let tone = 'text-teal-ink bg-teal-bg'
  if (!state.enrolled) {
    Icon = LogIn
    text = 'Not set up'
    tone = 'text-amber-ink bg-amber-bg'
  } else if (!state.connected) {
    Icon = AlertTriangle
    text = 'Disconnected'
    tone = 'text-red-ink bg-red-bg'
  }
  return (
    <span className={cx('inline-flex items-center gap-2 rounded-[10px] px-3 py-2 text-[12px] font-bold', tone)}>
      <Icon size={14} strokeWidth={2.4} />
      {text}
    </span>
  )
}

export function Header({
  title,
  state,
  search,
  onSearch,
  onToggleTheme,
}: {
  title: string
  state: StateDTO
  search: string
  onSearch: (v: string) => void
  onToggleTheme: () => void
}) {
  const dark = state.theme !== 'light'
  return (
    <header className="flex h-[66px] shrink-0 items-center gap-4 border-b border-border bg-base px-6">
      <h1 className="font-display text-[18px] font-bold text-text">{title}</h1>
      <div className="flex-1" />
      <SearchInput value={search} onChange={onSearch} />
      <ConnPill state={state} />
      <IconButton icon={dark ? Sun : Moon} onClick={onToggleTheme} title="Toggle theme" />
    </header>
  )
}
