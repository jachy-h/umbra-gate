interface BadgeProps {
  children: React.ReactNode
  color?: 'default' | 'orange' | 'pink' | 'violet' | 'emerald' | 'success' | 'warning' | 'error'
}

const colorMap: Record<string, string> = {
  default: 'bg-[var(--color-surface-card)] text-[var(--color-ink)]',
  orange: 'bg-[var(--color-badge-orange)] text-white',
  pink: 'bg-[var(--color-badge-pink)] text-white',
  violet: 'bg-[var(--color-badge-violet)] text-white',
  emerald: 'bg-[var(--color-badge-emerald)] text-white',
  success: 'bg-[var(--color-success)] text-white',
  warning: 'bg-[var(--color-warning)] text-white',
  error: 'bg-[var(--color-error)] text-white',
}

export function Badge({ children, color = 'default' }: BadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-[13px] font-medium leading-[1.4] ${colorMap[color]}`}>
      {children}
    </span>
  )
}
