interface NavPillGroupProps {
  items: { key: string; label: string }[]
  active: string
  onChange: (key: string) => void
}

export function NavPillGroup({ items, active, onChange }: NavPillGroupProps) {
  return (
    <div className="inline-flex items-center gap-1 rounded-full bg-[var(--color-surface-soft)] p-1.5">
      {items.map((item) => {
        const isActive = item.key === active
        return (
          <button
            key={item.key}
            onClick={() => onChange(item.key)}
            className={`rounded-md px-3.5 py-2 text-sm font-medium leading-[1.4] transition-all cursor-pointer ${
              isActive
                ? 'bg-[var(--color-canvas)] text-[var(--color-ink)] shadow-[0_1px_2px_rgba(0,0,0,0.05)]'
                : 'bg-transparent text-[var(--color-muted)] hover:text-[var(--color-ink)]'
            }`}
          >
            {item.label}
          </button>
        )
      })}
    </div>
  )
}
