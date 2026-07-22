import { type ButtonHTMLAttributes } from 'react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md'
}

export function Button({ variant = 'primary', size = 'md', className = '', children, disabled, ...props }: ButtonProps) {
  const base = 'inline-flex items-center justify-center font-semibold text-sm cursor-pointer transition-colors rounded-md'
  const sizing = size === 'sm' ? 'h-8 px-4 text-xs' : 'h-10 px-5'

  const variants: Record<string, string> = {
    primary: 'bg-[var(--color-primary)] text-[var(--color-on-primary)] hover:bg-[var(--color-primary-active)] disabled:bg-[var(--color-primary-disabled)] disabled:text-[var(--color-muted)]',
    secondary: 'bg-[var(--color-canvas)] text-[var(--color-ink)] border border-[var(--color-hairline)] hover:bg-[var(--color-surface-soft)]',
    ghost: 'bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)]',
    danger: 'bg-[var(--color-error)] text-white hover:opacity-90',
  }

  return (
    <button
      className={`${base} ${sizing} ${variants[variant]} ${className}`}
      disabled={disabled}
      {...props}
    >
      {children}
    </button>
  )
}
