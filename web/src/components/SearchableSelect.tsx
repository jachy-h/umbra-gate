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
  const listRef = useRef<HTMLDivElement>(null)

  const selectedLabel = useMemo(
    () => options.find((option) => option.value === value)?.label || '',
    [options, value],
  )

  const filtered = useMemo(() => {
    if (!query) return options
    const normalizedQuery = query.toLowerCase()
    return options.filter((option) => option.label.toLowerCase().includes(normalizedQuery))
  }, [options, query])

  useEffect(() => {
    setHighlightIndex(filtered.length > 0 ? 0 : -1)
  }, [filtered])

  useEffect(() => {
    function handleOutsideClick(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false)
        setQuery('')
      }
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => document.removeEventListener('mousedown', handleOutsideClick)
  }, [])

  useEffect(() => {
    if (highlightIndex >= 0 && listRef.current?.children[highlightIndex]) {
      listRef.current.children[highlightIndex].scrollIntoView({ block: 'nearest' })
    }
  }, [highlightIndex])

  const choose = useCallback((nextValue: string) => {
    onChange(nextValue)
    setOpen(false)
    setQuery('')
  }, [onChange])

  const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setOpen(true)
      setHighlightIndex((previous) => previous >= filtered.length - 1 ? 0 : previous + 1)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      setOpen(true)
      setHighlightIndex((previous) => previous <= 0 ? filtered.length - 1 : previous - 1)
    } else if (event.key === 'Enter' && open && highlightIndex >= 0) {
      event.preventDefault()
      choose(filtered[highlightIndex].value)
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
      setQuery('')
      event.currentTarget.blur()
    }
  }, [choose, filtered, highlightIndex, open])

  const inputClass = `h-10 w-full rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] pl-3.5 pr-9 text-sm text-[var(--color-ink)] outline-none transition-colors placeholder:text-[var(--color-muted-soft)] focus:border-[var(--color-ink)] ${
    disabled ? 'cursor-not-allowed opacity-50' : 'cursor-text'
  } ${className}`

  return (
    <div ref={containerRef} className="relative">
      <input
        type="text"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        autoComplete="off"
        value={open ? query : selectedLabel}
        onFocus={() => {
          if (!disabled) {
            setQuery('')
            setOpen(true)
          }
        }}
        onClick={() => !disabled && setOpen(true)}
        onChange={(event) => {
          setOpen(true)
          setQuery(event.target.value)
        }}
        onKeyDown={handleKeyDown}
        disabled={disabled}
        placeholder={placeholder}
        className={inputClass}
      />
      <svg
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="var(--color-muted)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={`pointer-events-none absolute right-3 top-3 transition-transform duration-150 ${open ? 'rotate-180' : ''}`}
      >
        <path d="M6 9l6 6 6-6" />
      </svg>

      {open && (
        <div className="absolute z-50 mt-1 w-full overflow-hidden rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] shadow-[0_4px_12px_rgba(0,0,0,0.08)] animate-fade-in">
          <div ref={listRef} role="listbox" className="max-h-52 overflow-y-auto">
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-center text-sm text-[var(--color-muted)]">No results found</div>
            ) : (
              filtered.map((option, index) => (
                <button
                  key={option.value}
                  type="button"
                  role="option"
                  aria-selected={option.value === value}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => choose(option.value)}
                  className={`w-full px-3 py-2.5 text-left text-sm transition-colors ${
                    index === highlightIndex
                      ? 'bg-blue-50 font-medium text-[var(--color-ink)] shadow-[inset_3px_0_0_var(--color-brand-accent)]'
                      : option.value === value
                        ? 'bg-[var(--color-surface-soft)] font-medium text-[var(--color-ink)]'
                        : 'text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)]'
                  }`}
                >
                  {option.label}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
