import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

// The consistent, pixel-accurate component kit, real DOM/CSS, so buttons,
// toggles, inputs and dialogs all share one system (the reason for the switch).

function cx(...parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(' ')
}

// --- Button (pill) ----------------------------------------------------------

type ButtonVariant = 'primary' | 'secondary' | 'danger'

const buttonVariants: Record<ButtonVariant, string> = {
  primary: 'bg-amber text-on-primary hover:brightness-95',
  secondary: 'bg-surface2 text-text border border-border2 hover:bg-border2/60',
  danger: 'bg-red-bg text-red-ink hover:brightness-110',
}

export function Button({
  children,
  onClick,
  variant = 'secondary',
  icon: Icon,
  disabled,
  full,
  className,
  type = 'button',
}: {
  children?: ReactNode
  onClick?: () => void
  variant?: ButtonVariant
  icon?: LucideIcon
  disabled?: boolean
  full?: boolean
  className?: string
  type?: 'button' | 'submit'
}) {
  return (
    <button
      type={type}
      disabled={disabled}
      onClick={onClick}
      className={cx(
        'inline-flex items-center justify-center gap-2 rounded-[10px] px-3.5 py-2 font-display text-[13px] font-bold transition',
        'disabled:cursor-not-allowed disabled:opacity-50',
        full && 'w-full',
        buttonVariants[variant],
        className,
      )}
    >
      {Icon && <Icon size={14} strokeWidth={2.4} />}
      {children}
    </button>
  )
}

// A square, bordered icon button (header theme toggle, compact affordances).
export function IconButton({
  icon: Icon,
  onClick,
  active,
  title,
}: {
  icon: LucideIcon
  onClick?: () => void
  active?: boolean
  title?: string
}) {
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className={cx(
        'flex h-[42px] w-[42px] items-center justify-center rounded-[11px] border border-border2 transition',
        active ? 'bg-amber text-on-primary' : 'bg-surface2 text-amber hover:bg-border2/60',
      )}
    >
      <Icon size={16} strokeWidth={2.2} />
    </button>
  )
}

// --- Toggle -----------------------------------------------------------------

export function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      onClick={() => onChange(!on)}
      className={cx(
        'relative h-[27px] w-[46px] shrink-0 rounded-full transition-colors',
        on ? 'bg-amber' : 'bg-surface2',
      )}
    >
      <span
        className={cx(
          'absolute top-[3px] h-[21px] w-[21px] rounded-full bg-white transition-all',
          on ? 'left-[22px]' : 'left-[3px]',
        )}
      />
    </button>
  )
}

// --- Status pill / dot ------------------------------------------------------

export function StatusPill({ text, tone }: { text: string; tone: { text: string; bg: string } }) {
  return (
    <span
      className={cx(
        'inline-flex items-center rounded-full px-2.5 py-[3px] text-[11px] font-extrabold tracking-wide',
        tone.text,
        tone.bg,
      )}
    >
      {text}
    </span>
  )
}

export function StatusDot({ text, tone }: { text: string; tone: { text: string; dot: string } }) {
  return (
    <span className={cx('inline-flex items-center gap-1.5 text-[12px] font-bold', tone.text)}>
      <span className={cx('h-2 w-2 rounded-full', tone.dot)} />
      {text}
    </span>
  )
}

// --- Chip -------------------------------------------------------------------

export function Chip({ icon: Icon, children }: { icon?: LucideIcon; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-lg bg-surface2 px-2.5 py-1 text-[11px] font-semibold text-muted">
      {Icon && <Icon size={12} strokeWidth={2.2} />}
      {children}
    </span>
  )
}

// --- Progress bar -----------------------------------------------------------

export function ProgressBar({
  value,
  accent = 'amber',
}: {
  value: number // 0..1, or <0 for indeterminate
  accent?: 'amber' | 'muted'
}) {
  const indeterminate = value < 0
  const pct = Math.max(0, Math.min(1, value)) * 100
  const track = accent === 'amber' ? 'bg-amber/25' : 'bg-muted/20'
  const fill = accent === 'amber' ? 'bg-amber' : 'bg-muted'
  return (
    <div className={cx('h-[7px] w-full overflow-hidden rounded-full', track)}>
      <div
        className={cx('h-full rounded-full transition-all', fill, indeterminate && 'animate-pulse')}
        style={{ width: indeterminate ? '35%' : `${pct}%` }}
      />
    </div>
  )
}

// --- Card -------------------------------------------------------------------

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cx('rounded-xl border border-border bg-surface', className)}>{children}</div>
  )
}

// --- Segmented control ------------------------------------------------------

export interface SegItem {
  key: string
  label: string
  icon?: LucideIcon
  count?: number
}

export function Segmented({
  items,
  value,
  onChange,
}: {
  items: SegItem[]
  value: string
  onChange: (key: string) => void
}) {
  return (
    <div className="inline-flex gap-1 rounded-[11px] border border-border2 bg-surface p-[5px]">
      {items.map((it) => {
        const active = it.key === value
        return (
          <button
            key={it.key}
            type="button"
            onClick={() => onChange(it.key)}
            className={cx(
              'inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-bold transition',
              active ? 'bg-amber text-on-primary' : 'text-muted hover:text-text',
            )}
          >
            {it.icon && <it.icon size={13} strokeWidth={2.2} />}
            {it.label}
            {it.count !== undefined && it.count >= 0 && <span className="opacity-70">{it.count}</span>}
          </button>
        )
      })}
    </div>
  )
}

export { cx }
