import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Provider, ProxyLink, ChainEntry, ProviderProtocol } from '../types'
import { Button } from '../components/Button'
import { Input } from '../components/Input'
import { SearchableSelect } from '../components/SearchableSelect'
import { Spinner } from '../components/Spinner'
import { protocolLabel } from '../protocols'

const blankChainEntry = (): ChainEntry => ({ provider_id: '', protocol: '', retry_count: 1, fallback_model: '', api_key: '' })

interface Props {
  link?: ProxyLink | null
  onSaved: () => void
  onCancel: () => void
}

export function LinkEditor({ link, onSaved, onCancel }: Props) {
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [chain, setChain] = useState<ChainEntry[]>([])
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const isEditing = !!link

  useEffect(() => {
    api.listProviders().then((p) => {
      setProviders(p)
      if (link) {
        setName(link.name)
        setPath(link.path)
        setEnabled(link.enabled)
        setChain(link.chain?.length
          ? link.chain.map((entry, index) => ({ ...entry, protocol: entry.protocol || (index === 0 ? link.protocol : '') }))
          : [blankChainEntry()])
      } else {
        setChain([blankChainEntry()])
      }
    }).catch(console.error).finally(() => setLoading(false))
  }, [link])

  const updateChain = (i: number, field: keyof ChainEntry, value: string | number) => {
    setChain((prev) => prev.map((c, idx) => idx === i ? { ...c, [field]: value } : c))
  }

  const addChainEntry = () => {
    setChain((prev) => [...prev, blankChainEntry()])
  }

  const removeChainEntry = (i: number) => {
    setChain((prev) => prev.length === 1 ? [blankChainEntry()] : prev.filter((_, idx) => idx !== i))
  }

  const moveChainEntry = (i: number, direction: 'up' | 'down') => {
    const newIndex = direction === 'up' ? i - 1 : i + 1
    if (newIndex < 0 || newIndex >= chain.length) return
    const reordered = [...chain]
    const [moved] = reordered.splice(i, 1)
    reordered.splice(newIndex, 0, moved)
    setChain(reordered)
  }

  const save = async () => {
    if (!chain[0]?.provider_id) {
      setError('Select a provider for the primary node. Step 1 must define the link protocol.')
      return
    }
    const selectedChain = chain.filter((entry) => entry.provider_id)
    const selectedProtocol = selectedChain[0]?.protocol || ''
    const mismatch = selectedChain.findIndex((entry) => !entry.protocol || entry.protocol !== selectedProtocol)
    if (!selectedProtocol) {
      setError('Select a protocol on the primary node. The first node defines the link protocol.')
      return
    }
    if (mismatch >= 0) {
      setError(`Protocol mismatch at chain step ${mismatch + 1}. Every node must use ${protocolLabel(selectedProtocol)}.`)
      return
    }
    setSaving(true)
    setError('')
    try {
      const payload: Partial<ProxyLink> = {
        name,
        path: path || undefined,
        protocol: selectedProtocol as ProviderProtocol,
        chain: selectedChain,
        enabled,
      }
      if (link) payload.id = link.id
      await api.createLink(payload)
      onSaved()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <Spinner />

  const fieldCls = 'h-10 px-3.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] text-sm outline-none transition-colors focus:border-[var(--color-ink)] w-full'
  const labelCls = 'text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]'

  const providerName = (id: string) => providers.find((p) => p.id === id)?.name || id
  const providerProtocols = (provider?: Provider) => Array.from(new Set((provider?.endpoints || []).map((endpoint) => endpoint.protocol)))
  const linkProtocol = chain[0]?.protocol || ''
  const protocolMismatchIndexes = new Set(chain.map((entry, index) => (
    index > 0 && !!entry.provider_id && (!entry.protocol || entry.protocol !== linkProtocol) ? index : -1
  )).filter((index) => index >= 0))

  const selectProvider = (index: number, providerID: string) => {
    const provider = providers.find((item) => item.id === providerID)
    const matchingLinkProtocol = index > 0 && linkProtocol
      ? provider?.endpoints?.find((endpoint) => endpoint.protocol === linkProtocol)?.protocol
      : undefined
    const protocol = matchingLinkProtocol || provider?.endpoints?.[0]?.protocol || ''
    setChain((current) => current.map((entry, entryIndex) => entryIndex === index
      ? { ...entry, provider_id: providerID, protocol }
      : entry))
  }

  return (
    <div className="animate-fade-in">
      <div className="mb-8">
        <h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
          {isEditing ? 'Edit Proxy Link' : 'New Proxy Link'}
        </h1>
        <p className="mt-2 text-[var(--color-muted)] text-base">
          Define a proxy route with provider chaining for automatic fallback.
        </p>
      </div>

      <div className="flex gap-8 items-start">
        {/* Left column: Basic info */}
        <div className="w-[360px] shrink-0 flex flex-col gap-6">
          <div className="rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-6 space-y-5">
            <Input
              label="Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Production Gateway"
            />

            <Input
              label="Path (auto if empty)"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="Leave blank for random token"
              disabled={isEditing}
            />

            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-[var(--color-ink)]">Enabled</span>
              <button
                type="button"
                onClick={() => setEnabled(!enabled)}
                className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                  enabled ? 'bg-[var(--color-primary)]' : 'bg-[var(--color-hairline)]'
                }`}
              >
                <span
                  className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                    enabled ? 'translate-x-5' : 'translate-x-0'
                  }`}
                />
              </button>
            </div>
          </div>

          <div className="rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)] p-6">
            <div className="mb-4 rounded-lg border-2 border-[var(--color-ink)] bg-[var(--color-ink)] px-4 py-3 text-white shadow-sm">
              <p className="text-[10px] font-bold uppercase tracking-[0.18em] text-white/60">Link protocol · fixed by step 1</p>
              <p className="mt-1 text-sm font-semibold">{protocolLabel(linkProtocol)}</p>
            </div>
            <h3 className="text-sm font-semibold text-[var(--color-ink)] mb-3">Chain Preview</h3>
            {chain.filter(c => c.provider_id).length === 0 ? (
              <p className="text-sm text-[var(--color-muted)]">No providers in chain yet.</p>
            ) : (
              <div className="flex items-center flex-wrap gap-1.5">
                {chain.filter(c => c.provider_id).map((c, i, arr) => (
                  <span key={i} className="flex items-center gap-1.5">
                    {i > 0 && (
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--color-muted)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0">
                        <path d="M5 12h14M13 5l7 7-7 7" />
                      </svg>
                    )}
                    <span className={`inline-flex items-center rounded-full px-3 py-1 text-[13px] font-medium leading-[1.4] ${
                      i === 0 ? 'bg-[var(--color-badge-violet)] text-white' :
                      i === arr.length - 1 ? 'bg-[var(--color-badge-emerald)] text-white' :
                      'bg-[var(--color-badge-orange)] text-white'
                    }`}>
                      {providerName(c.provider_id)}
                      <span className="ml-1 opacity-70">· {protocolLabel(c.protocol)}</span>
                      {!!c.api_key && ' *'}
                    </span>
                  </span>
                ))}
              </div>
            )}
          </div>

          <div className="flex gap-3">
            <Button variant="secondary" onClick={onCancel} className="flex-1">Cancel</Button>
            <Button onClick={save} disabled={saving || !name || chain.filter((c) => c.provider_id).length === 0} className="flex-1">
              {saving ? 'Saving...' : 'Save Link'}
            </Button>
          </div>

          {error && <p className="text-sm text-[var(--color-error)]">{error}</p>}
        </div>

        {/* Right column: Provider Chain */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-5">
            <h2 className="text-base font-semibold text-[var(--color-ink)]">Provider Chain</h2>
            <Button variant="secondary" size="sm" onClick={addChainEntry}>+ Add Step</Button>
          </div>

          <div className="space-y-3">
            {chain.map((entry, i) => {
              const provider = providers.find((p) => p.id === entry.provider_id)
              const hasGlobalKey = !!provider?.has_api_key
              const isFirst = i === 0
              const isLast = i === chain.length - 1
              return (
                <div key={i} className={`relative rounded-xl border bg-[var(--color-canvas)] transition-colors ${
                  protocolMismatchIndexes.has(i) ? 'border-red-500 shadow-[0_0_0_3px_rgba(239,68,68,0.10)]' : 'border-[var(--color-hairline)]'
                }`}>
                  {/* Header */}
                  <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--color-hairline-soft)]">
                    <div className="flex items-center gap-3">
                      <span className={`flex items-center justify-center w-7 h-7 rounded-full text-xs font-bold text-white ${
                        isFirst ? 'bg-[var(--color-badge-violet)]' :
                        isLast ? 'bg-[var(--color-badge-emerald)]' :
                        'bg-[var(--color-badge-orange)]'
                      }`}>
                        {i + 1}
                      </span>
                      <span className="text-sm font-semibold text-[var(--color-ink)]">
                        {isFirst ? 'Primary' : isLast ? 'Final Fallback' : `Fallback #${i}`}
                      </span>
                      {entry.protocol && (
                        <span className={`rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${
                          protocolMismatchIndexes.has(i) ? 'bg-red-600 text-white' : 'bg-[var(--color-ink)] text-white'
                        }`}>
                          {protocolLabel(entry.protocol)}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {!isFirst && (
                        <button
                          onClick={() => moveChainEntry(i, 'up')}
                          className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)] transition-colors cursor-pointer"
                          title="Move up"
                        >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M18 15l-6-6-6 6" />
                          </svg>
                        </button>
                      )}
                      {!isLast && (
                        <button
                          onClick={() => moveChainEntry(i, 'down')}
                          className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface-soft)] transition-colors cursor-pointer"
                          title="Move down"
                        >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M6 9l6 6 6-6" />
                          </svg>
                        </button>
                      )}
                      <button
                        onClick={() => removeChainEntry(i)}
                        className="inline-flex items-center justify-center w-7 h-7 rounded-md text-[var(--color-error)] hover:bg-[var(--color-error)]/10 transition-colors cursor-pointer"
                        title="Remove"
                      >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6h14" />
                          </svg>
                      </button>
                    </div>
                  </div>

                  {/* Body */}
                  <div className="p-5 space-y-4">
                    {isFirst && (
                      <div className="flex items-start gap-3 rounded-lg border border-[var(--color-hairline)] bg-[var(--color-surface-soft)] px-4 py-3">
                        <span className="mt-0.5 h-2.5 w-2.5 shrink-0 rounded-full bg-[var(--color-ink)]" />
                        <div>
                          <p className="text-xs font-semibold text-[var(--color-ink)]">This node defines the protocol for the entire link.</p>
                          <p className="mt-0.5 text-xs text-[var(--color-muted)]">Changing it will mark incompatible fallback nodes in red.</p>
                        </div>
                      </div>
                    )}
                    {protocolMismatchIndexes.has(i) && (
                      <div className="rounded-lg bg-red-600 px-4 py-3 text-sm font-semibold text-white">
                        Protocol mismatch — this node uses {protocolLabel(entry.protocol)}, while the link requires {protocolLabel(linkProtocol)}. Saving will fail.
                      </div>
                    )}
                    <div className="grid grid-cols-12 gap-4">
                      <div className="col-span-4 space-y-1.5">
                        <label className={labelCls}>Provider</label>
                        <SearchableSelect
                          options={providers.map((p) => ({ label: `${p.name} · ${providerProtocols(p).map(protocolLabel).join(' / ')}`, value: p.id }))}
                          value={entry.provider_id}
                          onChange={(v) => selectProvider(i, v)}
                          placeholder="Search provider..."
                        />
                      </div>
                      <div className="col-span-3 space-y-1.5">
                        <label className={labelCls}>{isFirst ? 'Link Protocol' : 'Protocol'}</label>
                        <SearchableSelect
                          options={providerProtocols(provider).map((protocol) => ({ label: protocolLabel(protocol), value: protocol }))}
                          value={entry.protocol}
                          onChange={(value) => updateChain(i, 'protocol', value)}
                          disabled={!provider}
                          placeholder="Select protocol…"
                          className={protocolMismatchIndexes.has(i) ? '!border-red-500 !text-red-700' : ''}
                        />
                      </div>
                      <div className="col-span-2 space-y-1.5">
                        <label className={labelCls}>Retry</label>
                        <input
                          type="number"
                          value={String(entry.retry_count)}
                          onChange={(e) => updateChain(i, 'retry_count', parseInt(e.target.value) || 0)}
                          className={fieldCls}
                          min={0}
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-1.5">
                        <label className={labelCls}>Fallback Model</label>
                        <input
                          value={entry.fallback_model}
                          onChange={(e) => updateChain(i, 'fallback_model', e.target.value)}
                          placeholder="optional"
                          className={fieldCls}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <label className={labelCls}>API Key {hasGlobalKey && '(override global)'}</label>
                        <input
                          type="text"
                          autoComplete="off"
                          name={`link-api-key-${i}`}
                          value={entry.api_key || ''}
                          onChange={(e) => updateChain(i, 'api_key', e.target.value)}
                          placeholder={hasGlobalKey ? 'Leave empty to use global key' : 'sk-...'}
                          className={fieldCls}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}
