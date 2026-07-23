import { useEffect, useState } from 'react'
import { api } from '../api'
import type { ProxyLink, Provider, RequestLog } from '../types'
import { useHash } from '../hooks/useHash'
import { Card } from '../components/Card'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Spinner } from '../components/Spinner'
import { RequestDetailsModal } from '../components/RequestDetailsModal'
import { protocolLabel } from '../protocols'

export function LinkManager() {
  const [links, setLinks] = useState<ProxyLink[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [requests, setRequests] = useState<RequestLog[]>([])
  const [selectedRequest, setSelectedRequest] = useState<RequestLog | null>(null)
  const [loading, setLoading] = useState(true)
  const [testingLinkID, setTestingLinkID] = useState<string | null>(null)
  const { navigate } = useHash()

  const fetchAll = () => {
    setLoading(true)
    return Promise.all([api.listLinks(), api.listProviders(), api.listValidationRequests()])
      .then(([l, p, r]) => {
        setLinks(l)
        setProviders(p)
        setRequests(r)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchAll() }, [])

  const remove = async (id: string) => {
    if (!confirm('Delete this proxy link?')) return
    try {
      await api.deleteLink(id)
      fetchAll()
    } catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed') }
  }

  const testLink = async (id: string) => {
    setTestingLinkID(id)
    try {
      await api.testLink(id)
      await fetchAll()
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : 'Link test failed')
    } finally {
      setTestingLinkID(null)
    }
  }

  const gatewayBase = (import.meta.env.VITE_GATEWAY_BASE as string | undefined)?.replace(/\/$/, '') || `http://${window.location.hostname}:8787`

  const proxyUrl = (path: string) => `${gatewayBase}/llm-gateway-lite/${path}`

  const copyUrl = async (path: string) => {
    const url = proxyUrl(path)
    try {
      await navigator.clipboard.writeText(url)
    } catch {
      const el = document.createElement('textarea')
      el.value = url
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
  }

  const providerName = (id: string) => providers.find((p) => p.id === id)?.name || id

  const validationRequest = (linkId: string, providerId: string, position: number) => {
    const matching = requests.filter((request) =>
      request.link_id === linkId &&
      request.provider_id === providerId &&
      request.attributes?._request_type === 'link_validation'
    )
    return matching.find((request) => Number(request.attributes?._chain_position) === position) || matching[0]
  }

  return (
    <div className="space-y-8 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
            Proxy Links
          </h1>
          <p className="mt-2 text-[var(--color-muted)] text-base">
            Configure proxy routes with provider chaining for fallback.
          </p>
        </div>
        <Button onClick={() => navigate('/links/new')}>+ New Link</Button>
      </div>

      {loading ? (
        <Spinner />
      ) : links.length === 0 ? (
        <Card className="text-center text-[var(--color-muted)] py-16">
          No proxy links configured. Create one to start routing requests.
        </Card>
      ) : (
        <div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
                <th className="px-8 py-3 font-medium">Name</th>
                <th className="px-8 py-3 font-medium">Protocol</th>
                <th className="px-8 py-3 font-medium">Proxy URL</th>
                <th className="px-8 py-3 font-medium">Chain</th>
                <th className="px-8 py-3 font-medium w-48" />
              </tr>
            </thead>
            <tbody>
              {links.map((l) => (
                <tr key={l.id} className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors">
                  <td className="px-8 py-4">
                    <div className="text-sm font-semibold text-[var(--color-ink)]">{l.name}</div>
                  </td>
                  <td className="px-8 py-4 text-sm text-[var(--color-muted)]">
                    {protocolLabel(l.protocol)}
                  </td>
                  <td className="px-8 py-4 text-sm">
                    <div className="flex items-center gap-2">
                      <code
                        className="font-mono text-xs text-[var(--color-muted)] bg-[var(--color-surface-soft)] px-2 py-1 rounded truncate block max-w-xs"
                        title={proxyUrl(l.path)}
                      >
                        {proxyUrl(l.path)}
                      </code>
                      <button
                        onClick={() => copyUrl(l.path)}
                        className="inline-flex items-center justify-center w-6 h-6 rounded-md cursor-pointer hover:bg-[var(--color-surface-soft)] text-[var(--color-muted)] hover:text-[var(--color-ink)] transition-colors"
                        title="Copy URL"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                        </svg>
                      </button>
                    </div>
                  </td>
                  <td className="px-8 py-4 text-sm">
                    <div className="flex items-center flex-wrap gap-1.5">
                      {l.chain?.map((c, i) => {
                        const hasOverride = !!c.api_key
                        const failed = c.validation_ok === false
                        const protocolMismatch = c.protocol !== l.protocol
                        const testRequest = validationRequest(l.id, c.provider_id, i)
                        return (
                          <span key={i} className="flex items-center gap-1.5">
                            {i > 0 && (
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--color-muted)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0">
                                <path d="M5 12h14M13 5l7 7-7 7" />
                              </svg>
                            )}
                            <button
                              type="button"
                              onClick={() => testRequest && setSelectedRequest(testRequest)}
                              className={`rounded-full text-left transition-opacity ${failed && !protocolMismatch ? 'grayscale opacity-40' : ''} ${testRequest ? 'cursor-pointer hover:opacity-75' : 'cursor-default'}`}
                              title={protocolMismatch ? `Protocol mismatch: link requires ${protocolLabel(l.protocol)}` : testRequest ? 'View Link Test request' : failed ? c.validation_error || 'Validation failed' : 'No Link Test request recorded'}
                            >
                              <Badge color={protocolMismatch ? 'error' : i === 0 ? 'violet' : i === l.chain!.length - 1 ? 'emerald' : 'orange'}>
                                {providerName(c.provider_id)}
                                {hasOverride && ' *'}
                              </Badge>
                            </button>
                          </span>
                        )
                      })}
                    </div>
                  </td>
                  <td className="px-8 py-4">
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => testLink(l.id)}
                        disabled={testingLinkID === l.id}
                        title="Run a Link Test against every provider in this chain"
                      >
                        {testingLinkID === l.id ? 'Testing…' : 'Test'}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => navigate(`/links/edit/${l.id}`)}>Edit</Button>
                      <Button variant="ghost" size="sm" onClick={() => remove(l.id)} className="!text-[var(--color-error)]">Del</Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <RequestDetailsModal request={selectedRequest} onClose={() => setSelectedRequest(null)} />
    </div>
  )
}
