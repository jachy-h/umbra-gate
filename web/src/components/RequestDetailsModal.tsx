import type { RequestLog } from '../types'
import { Modal } from './Modal'

interface Props {
  request: RequestLog | null
  onClose: () => void
}

function formatPayload(payload?: string) {
  if (!payload) return 'No content recorded.'
  try {
    return JSON.stringify(JSON.parse(payload), null, 2)
  } catch {
    return payload
  }
}

function formatHeaders(headers?: Record<string, unknown>) {
  if (!headers || Object.keys(headers).length === 0) return 'No headers recorded.'
  return JSON.stringify(headers, null, 2)
}

export function RequestDetailsModal({ request, onClose }: Props) {
  return (
    <Modal open={request !== null} title="Request Details" onClose={onClose} className="!max-w-6xl max-h-[88vh] overflow-y-auto">
      {request && (
        <div className="space-y-5">
          <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-[var(--color-muted)]">
            <span><strong className="text-[var(--color-ink)]">Provider:</strong> {request.provider_name}</span>
            <span><strong className="text-[var(--color-ink)]">Model:</strong> {request.model || '—'}</span>
            <span><strong className="text-[var(--color-ink)]">Status:</strong> {request.status_code || 'ERR'}</span>
            <span><strong className="text-[var(--color-ink)]">Latency:</strong> {request.latency_ms}ms</span>
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <section className="min-w-0 space-y-5">
              <h3 className="text-base font-semibold text-[var(--color-ink)]">Request</h3>
              {request.request_url && (
                <div className="space-y-2">
                  <h4 className="text-sm font-semibold text-[var(--color-ink)]">Agent → Gateway</h4>
                  <p className="break-all rounded-lg bg-[var(--color-surface-soft)] px-4 py-3 text-xs font-mono">{request.request_url}</p>
                  <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Headers</p>
                  <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatHeaders(request.request_headers)}</pre>
                  <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Body</p>
                  <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatPayload(request.request_body)}</pre>
                </div>
              )}
              <div className="space-y-2">
                <h4 className="text-sm font-semibold text-[var(--color-ink)]">Gateway → Provider</h4>
                <p className="break-all rounded-lg bg-[var(--color-surface-soft)] px-4 py-3 text-xs font-mono">{request.upstream_url || 'No upstream URL recorded.'}</p>
                <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Headers</p>
                <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatHeaders(request.upstream_headers)}</pre>
                <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Body</p>
                <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatPayload(request.upstream_body || request.request_body)}</pre>
              </div>
            </section>
            <section className="min-w-0 space-y-4 lg:border-l lg:border-[var(--color-hairline)] lg:pl-6">
              <h3 className="text-base font-semibold text-[var(--color-ink)]">Response</h3>
              {request.error_message && <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-[var(--color-error)]">{request.error_message}</div>}
              <div className="space-y-2">
                <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Headers</p>
                <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatHeaders(request.response_headers)}</pre>
                <p className="text-[11px] font-medium uppercase tracking-wide text-[var(--color-muted)]">Body</p>
                <pre className="max-h-[62vh] overflow-auto whitespace-pre-wrap break-words rounded-lg bg-[var(--color-surface-soft)] p-4 text-xs font-mono">{formatPayload(request.response_body)}</pre>
              </div>
            </section>
          </div>
        </div>
      )}
    </Modal>
  )
}
