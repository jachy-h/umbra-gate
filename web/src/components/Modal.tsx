interface ModalProps {
  open: boolean
  title: string
  children: React.ReactNode
  onClose: () => void
}

export function Modal({ open, title, children, onClose }: ModalProps) {
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center animate-fade-in">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative w-full max-w-lg rounded-xl bg-[var(--color-canvas)] p-8 shadow-[0_4px_12px_rgba(0,0,0,0.08)] animate-slide-up">
        <h2 className="text-[22px] font-semibold leading-[1.3] tracking-[-0.3px] text-[var(--color-ink)] mb-6">
          {title}
        </h2>
        {children}
      </div>
    </div>
  )
}
