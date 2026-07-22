import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Provider } from '../types'
import { Card } from '../components/Card'
import { Button } from '../components/Button'
import { Input, Select } from '../components/Input'
import { Modal } from '../components/Modal'
import { Spinner } from '../components/Spinner'

export function ProviderManager() {
  const [providers, setProviders] = useState<Provider[]>([])
  const [types, setTypes] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Provider | null>(null)
  const [form, setForm] = useState({ name: '', type: 'custom', base_url: '', api_key: '', models: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [keyEditingId, setKeyEditingId] = useState<string | null>(null)
  const [keyValue, setKeyValue] = useState('')

  const fetchAll = () => {
    setLoading(true)
    Promise.all([api.listProviders(), api.getTypes()])
      .then(([p, t]) => {
        setProviders(p)
        setTypes(t.types)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchAll() }, [])

  const openCreate = () => {
    setEditing(null)
    setForm({ name: '', type: 'custom', base_url: '', api_key: '', models: '' })
    setError('')
    setModalOpen(true)
  }

  const openEdit = (p: Provider) => {
    setEditing(p)
    setForm({ name: p.name, type: p.type, base_url: p.base_url, api_key: '', models: (p.models || []).join(', ') })
    setError('')
    setModalOpen(true)
  }

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const payload: Partial<Provider> = {
        name: form.name,
        type: form.type,
        base_url: form.base_url,
        api_key: form.api_key || '',
        models: form.models.split(',').map((s) => s.trim()).filter(Boolean),
        enabled: true,
      }
      if (editing) {
        payload.id = editing.id
      }
      await api.createProvider(payload)
      setModalOpen(false)
      fetchAll()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const startKeyEdit = (p: Provider) => {
    setKeyEditingId(p.id)
    setKeyValue(p.api_key || '')
  }

  const saveKey = async (p: Provider) => {
    setSaving(true)
    try {
      await api.createProvider({
        id: p.id,
        name: p.name,
        type: p.type,
        base_url: p.base_url,
        api_key: keyValue,
        models: p.models || [],
        enabled: p.enabled,
      })
      setKeyEditingId(null)
      fetchAll()
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (id: string) => {
    if (!confirm('Delete this provider?')) return
    try {
      await api.deleteProvider(id)
      fetchAll()
    } catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed') }
  }

  const typeOptions = types.map((t) => ({ label: t, value: t }))

  return (
    <div className="space-y-8 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
            Providers
          </h1>
          <p className="mt-2 text-[var(--color-muted)] text-base">
            Manage upstream LLM provider configurations.
          </p>
        </div>
        <Button onClick={openCreate}>+ Add Provider</Button>
      </div>

      {loading ? (
        <Spinner />
      ) : providers.length === 0 ? (
        <Card className="text-center text-[var(--color-muted)] py-16">
          No providers configured. Add one to get started.
        </Card>
      ) : (
        <div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
                <th className="px-8 py-3 font-medium">Name</th>
                <th className="px-8 py-3 font-medium">Base URL</th>
                <th className="px-8 py-3 font-medium">API Key</th>
                <th className="px-8 py-3 font-medium w-24" />
              </tr>
            </thead>
            <tbody>
              {providers.map((p) => (
                <tr key={p.id} className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors">
                  <td className="px-8 py-4">
                    <span className="text-sm font-semibold text-[var(--color-ink)]">{p.name}</span>
                    {p.builtin && <span className="ml-2"><span className="inline-flex items-center rounded-full px-2.5 py-0.5 text-[13px] font-medium bg-[var(--color-surface-card)] text-[var(--color-muted)]">Built-in</span></span>}
                  </td>
                  <td className="px-8 py-4 text-sm text-[var(--color-muted)] max-w-[300px] truncate">{p.base_url}</td>
                  <td className="px-8 py-4">
                    {keyEditingId === p.id ? (
                      <div className="flex items-center gap-2">
                        <input
                          type="password"
                          value={keyValue}
                          onChange={(e) => setKeyValue(e.target.value)}
                          className="h-8 w-48 px-2.5 rounded-md border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-sm outline-none focus:border-[var(--color-ink)]"
                          placeholder="sk-..."
                          autoFocus
                          onKeyDown={(e) => { if (e.key === 'Enter') saveKey(p); if (e.key === 'Escape') setKeyEditingId(null) }}
                        />
                        <Button size="sm" onClick={() => saveKey(p)} disabled={saving}>Save</Button>
                        <Button size="sm" variant="ghost" onClick={() => setKeyEditingId(null)}>Cancel</Button>
                      </div>
                    ) : (
                      <button
                        onClick={() => startKeyEdit(p)}
                        className="text-sm text-[var(--color-muted)] cursor-pointer hover:text-[var(--color-ink)] transition-colors"
                        title={p.has_api_key ? 'Change API key' : 'Set API key'}
                      >
                        {p.has_api_key ? '••••••••' : 'Not set'}
                      </button>
                    )}
                  </td>
                  <td className="px-8 py-4">
                    {!p.builtin && (
                      <div className="flex gap-2">
                        <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>Edit</Button>
                        <Button variant="ghost" size="sm" onClick={() => remove(p.id)} className="!text-[var(--color-error)]">Del</Button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={modalOpen} title={editing ? 'Edit Provider' : 'New Provider'} onClose={() => setModalOpen(false)}>
        <div className="space-y-4">
          <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="e.g. My OpenAI" />
          <Select label="Type" options={typeOptions} value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })} />
          <Input label="Base URL" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} placeholder="https://api.openai.com" />
          <Input label="API Key" type="password" value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} placeholder="sk-..." />
          <Input label="Models" value={form.models} onChange={(e) => setForm({ ...form, models: e.target.value })} placeholder="gpt-4o, gpt-4o-mini" />
          {error && <p className="text-sm text-[var(--color-error)]">{error}</p>}
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setModalOpen(false)}>Cancel</Button>
            <Button onClick={save} disabled={saving || !form.name || !form.base_url}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
