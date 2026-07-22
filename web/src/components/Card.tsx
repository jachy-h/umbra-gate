interface CardProps {
  children: React.ReactNode
  className?: string
  variant?: 'default' | 'dark'
}

export function Card({ children, className = '', variant = 'default' }: CardProps) {
  const bg = variant === 'dark'
    ? 'bg-[var(--color-surface-dark)] text-[var(--color-on-dark)]'
    : 'bg-[var(--color-surface-card)] text-[var(--color-ink)]'

  return (
    <div className={`rounded-xl p-8 ${bg} ${className}`}>
      {children}
    </div>
  )
}
