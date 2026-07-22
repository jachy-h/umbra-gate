import { useState, useRef, useEffect, useMemo, useCallback } from 'react'

interface Option {
  label: string
  value: string
}

interface SearchableSelectProps {
  options: Option[]
  value: string
  onChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  className?: string
}

export function SearchableSelect({
  options,
  value,
  onChange,
  placeholder = 'Search...',
  disabled,
  className = '',
}: SearchableSelectProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [highlightIndex, setHighlightIndex] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const selectedLabel = useMemo(() => {
    return options.find((o) => o.value === value)?.label || ''
  }, [options, value])

  const filtered = useMemo(() => {
    if (!query) return options
    const q = query.toLowerCase()
    return options.filter((o) => o.label.toLowerCase().includes(q))
  }, [options, query])

  useEffect(() => {
    if (open) {
      setQuery('')
      setHighlightIndex(0)
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [open])

  useEffect(() => {
    setHighlightIndex(0)
  }, [query])

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const selectHighlighted = useCallback(() => {
    if (highlightIndex >= 0 && highlightIndex < filtered.length) {
      onChange(filtered[highlightIndex].value)
      setOpen(false)
    }
  }, [highlightIndex, filtered, onChange])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlightIndex((prev) => {
        if (prev >= filtered.length - 1) return 0
        return prev + 1
      })
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlightIndex((prev) => {
        if (prev <= 0) return filtered.length - 1
        return prev - 1
      })
    } else if (e.key === 'Enter') {
      e.preventDefault()
      selectHighlighted()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
    }
  }, [filtered.length, selectHighlighted])

  useEffect(() => {
    if (highlightIndex >= 0 && listRef.current) {
      const items = listRef.current.children
      if (items[highlightIndex]) {
        items[highlightIndex].scrollIntoView({ block: 'nearest' })
      }
    }
  }, [highlightIndex])

  const triggerCls = `h-10 px-3.5 w-full flex items-center justify-between rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-sm outline-none transition-colors focus-within:border-[var(--color-ink)] ${
    disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
  } ${className}`

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => !disabled && setOpen(!open)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault()
            if (!open) setOpen(true)
          }
        }}
        disabled={disabled}
        className={triggerCls}
      >
        <span className={selectedLabel ? 'text-[var(--color-ink)]' : 'text-[var(--color-muted)]'}>
          {selectedLabel || placeholder}
        </span>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="var(--color-muted)"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={`shrink-0 transition-transform duration-150 ${open ? 'rotate-180' : ''}`}
        >
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] shadow-[0_4px_12px_rgba(0,0,0,0.08)] overflow-hidden animate-fade-in">
          <div className="p-2 border-b border-[var(--color-hairline-soft)]">
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={placeholder}
              className="h-8 w-full px-2.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-sm text-[var(--color-ink)] outline-none placeholder:text-[var(--color-muted-soft)] focus:border-[var(--color-ink)]"
            />
          </div>
          <div ref={listRef} className="max-h-52 overflow-y-auto">
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-sm text-center text-[var(--color-muted)]">
                No results found
              </div>
            ) : (
              filtered.map((opt, idx) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => {
                    onChange(opt.value)
                    setOpen(false)
                  }}
                  className={`w-full px-3 py-2.5 text-sm text-left transition-colors ${
                    idx === highlightIndex
                      ? 'shadow-[inset_3px_0_0_var(--color-brand-accent)] bg-blue-50 text-[var(--color-ink)] font-medium'
                      : opt.value === value
                      ? 'bg-[var(--color-surface-soft)] text-[var(--color-ink)] font-medium'
                      : 'text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)]'
                  }`}
                >
                  {opt.label}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
