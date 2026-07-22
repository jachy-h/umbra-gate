import { type InputHTMLAttributes } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
}

export function Input({ label, className = '', id, ...props }: InputProps) {
  const inputId = id || label?.toLowerCase().replace(/\s+/g, '-')
  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={inputId} className="text-sm font-medium text-[var(--color-ink)]">
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`h-10 px-3.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] text-base outline-none transition-colors placeholder:text-[var(--color-muted-soft)] focus:border-[var(--color-ink)] ${className}`}
        {...props}
      />
    </div>
  )
}

interface SelectProps extends InputHTMLAttributes<HTMLSelectElement> {
  label?: string
  options: { label: string; value: string }[]
}

export function Select({ label, options, className = '', id, ...props }: SelectProps) {
  const selectId = id || label?.toLowerCase().replace(/\s+/g, '-')
  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={selectId} className="text-sm font-medium text-[var(--color-ink)]">
          {label}
        </label>
      )}
      <select
        id={selectId}
        className={`h-10 px-3.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] text-base outline-none transition-colors focus:border-[var(--color-ink)] ${className}`}
        {...props}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </div>
  )
}
