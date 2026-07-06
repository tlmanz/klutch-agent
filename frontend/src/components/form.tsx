import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Check, ChevronDown, Search, X, type LucideIcon } from 'lucide-react'
import { Button, cx } from './primitives'

// --- Text input -------------------------------------------------------------

export function TextInput({
  value,
  onChange,
  placeholder,
  icon: Icon,
  className,
  autoFocus,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  icon?: LucideIcon
  className?: string
  autoFocus?: boolean
}) {
  return (
    <div
      className={cx(
        'flex items-center gap-2.5 rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5',
        'focus-within:border-amber/60',
        className,
      )}
    >
      {Icon && <Icon size={16} className="shrink-0 text-muted" strokeWidth={2.2} />}
      <input
        autoFocus={autoFocus}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="w-full bg-transparent text-[14px] text-text placeholder:text-muted2 focus:outline-none"
      />
    </div>
  )
}

export function SearchInput(props: { value: string; onChange: (v: string) => void }) {
  return (
    <TextInput
      {...props}
      icon={Search}
      placeholder="Search printers & jobs"
      className="w-[320px] py-2"
    />
  )
}

export function TextArea({
  value,
  onChange,
  placeholder,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <textarea
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      rows={3}
      className="w-full resize-none rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5 text-[14px] text-text placeholder:text-muted2 focus:border-amber/60 focus:outline-none"
    />
  )
}

// --- Select (custom dropdown, matches the design) ---------------------------

export function Select({
  value,
  options,
  onChange,
  placeholder = 'Select…',
  className,
}: {
  value: string
  options: string[]
  onChange: (v: string) => void
  placeholder?: string
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])
  return (
    <div ref={ref} className={cx('relative', className)}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between gap-2 rounded-[11px] border border-border2 bg-surface px-3.5 py-2.5 text-[13px] font-semibold text-text"
      >
        <span className={cx(!value && 'text-muted2')}>{value || placeholder}</span>
        <ChevronDown size={15} className="text-muted" />
      </button>
      {open && (
        <div className="absolute right-0 z-20 mt-1.5 max-h-60 w-full min-w-[200px] overflow-auto rounded-[11px] border border-border2 bg-surface2 p-1.5 shadow-xl">
          {options.map((opt) => (
            <button
              key={opt}
              type="button"
              onClick={() => {
                onChange(opt)
                setOpen(false)
              }}
              className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-[13px] font-semibold text-text hover:bg-border2/60"
            >
              {opt}
              {opt === value && <Check size={14} className="text-amber" />}
            </button>
          ))}
          {options.length === 0 && (
            <div className="px-3 py-2 text-[13px] text-muted2">No printers</div>
          )}
        </div>
      )}
    </div>
  )
}

// --- Modal ------------------------------------------------------------------

export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  width = 460,
}: {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer?: ReactNode
  width?: number
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    if (open) document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-6">
      <div
        className="flex max-h-[90vh] flex-col overflow-hidden rounded-2xl border border-border2 bg-base shadow-2xl"
        style={{ width }}
      >
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="font-display text-[16px] font-bold text-text">{title}</h2>
          <button type="button" onClick={onClose} className="text-muted hover:text-text">
            <X size={18} />
          </button>
        </div>
        <div className="flex-1 overflow-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex justify-end gap-2 border-t border-border px-5 py-4">{footer}</div>
        )}
      </div>
    </div>
  )
}

// ConfirmModal is a simple yes/no dialog (replaces Fyne's dialog.ShowConfirm).
export function ConfirmModal({
  open,
  onClose,
  onConfirm,
  title,
  body,
  confirmLabel = 'Confirm',
  danger,
}: {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  body: string
  confirmLabel?: string
  danger?: boolean
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            onClick={() => {
              onConfirm()
              onClose()
            }}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p className="text-[14px] leading-relaxed text-muted">{body}</p>
    </Modal>
  )
}
