import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import type { StatsRow, ProxyLink } from '../types'
import { Card } from '../components/Card'
import { Badge } from '../components/Badge'
import { Spinner } from '../components/Spinner'
import { Input } from '../components/Input'
import { Button } from '../components/Button'

function fmtNum(n: number) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return String(n)
}

function fmtMs(ms: number, count: number) {
  if (count === 0) return '—'
  return (ms / count).toFixed(0) + 'ms'
}

function fmtRate(success: number, total: number) {
  if (total === 0) return '—'
  return ((success / total) * 100).toFixed(1) + '%'
}

export function StatsDashboard() {
  const [stats, setStats] = useState<StatsRow[]>([])
  const [links, setLinks] = useState<ProxyLink[]>([])
  const [loading, setLoading] = useState(true)
  const [linkFilter, setLinkFilter] = useState('')
  const [from, setFrom] = useState(() => {
    const d = new Date()
    d.setDate(d.getDate() - 7)
    return d.toISOString().slice(0, 10)
  })
  const [to, setTo] = useState(() => new Date().toISOString().slice(0, 10))

  const fetchData = () => {
    setLoading(true)
    Promise.all([
      api.getStats({ link_id: linkFilter || undefined, from: from ? `${from}T00:00:00` : undefined, to: to ? `${to}T23:59:59` : undefined }),
      api.listLinks(),
    ])
      .then(([s, l]) => {
        setStats(s.stats || [])
        setLinks(l)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  const aggregated = useMemo(() => {
    const byLink: Record<string, { total: number; success: number; failure: number; lat: number; name: string }> = {}
    let grandTotal = 0
    let grandSuccess = 0
    let grandFailure = 0
    let grandLat = 0

    for (const s of stats) {
      if (s.AttrKey !== '') continue // skip attribute-level rows, use link-level aggregates
      grandTotal += s.Total
      grandSuccess += s.Success
      grandFailure += s.Failure
      grandLat += s.Lat

      if (!byLink[s.LinkID]) {
        const link = links.find((l) => l.id === s.LinkID)
        byLink[s.LinkID] = { total: 0, success: 0, failure: 0, lat: 0, name: link?.name || s.LinkID }
      }
      byLink[s.LinkID].total += s.Total
      byLink[s.LinkID].success += s.Success
      byLink[s.LinkID].failure += s.Failure
      byLink[s.LinkID].lat += s.Lat
    }

    return { byLink, grandTotal, grandSuccess, grandFailure, grandLat }
  }, [stats, links])

  return (
    <div className="space-y-8 animate-fade-in">
      <div>
        <h1 className="text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">
          Statistics
        </h1>
        <p className="mt-2 text-[var(--color-muted)] text-base">
          Request volume, latency, and success rates across all proxy links.
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-end gap-4">
        <Input
          label="Link"
          value={linkFilter}
          onChange={(e) => setLinkFilter(e.target.value)}
          placeholder="All links"
          className="w-48"
        />
        <Input
          label="From"
          type="date"
          value={from}
          onChange={(e) => setFrom(e.target.value)}
        />
        <Input
          label="To"
          type="date"
          value={to}
          onChange={(e) => setTo(e.target.value)}
        />
        <Button onClick={fetchData}>Apply</Button>
      </div>

      {loading ? (
        <Spinner />
      ) : (
        <>
          {/* Overview cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            <Card>
              <p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">Total Requests</p>
              <p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">{fmtNum(aggregated.grandTotal)}</p>
            </Card>
            <Card>
              <p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">Success Rate</p>
              <p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-success)]">{fmtRate(aggregated.grandSuccess, aggregated.grandTotal)}</p>
            </Card>
            <Card>
              <p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">Failures</p>
              <p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-error)]">{fmtNum(aggregated.grandFailure)}</p>
            </Card>
            <Card>
              <p className="text-sm font-medium text-[var(--color-muted)] uppercase tracking-wide">Avg Latency</p>
              <p className="mt-2 text-[28px] font-semibold leading-[1.2] tracking-[-0.5px] text-[var(--color-ink)]">{fmtMs(aggregated.grandLat, aggregated.grandTotal)}</p>
            </Card>
          </div>

          {/* Per-link table */}
          {Object.keys(aggregated.byLink).length > 0 && (
            <div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
              <div className="px-8 py-5 border-b border-[var(--color-hairline-soft)]">
                <h3 className="text-lg font-semibold text-[var(--color-ink)]">By Proxy Link</h3>
              </div>
              <table className="w-full">
                <thead>
                  <tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
                    <th className="px-8 py-3 font-medium">Link</th>
                    <th className="px-8 py-3 font-medium">Total</th>
                    <th className="px-8 py-3 font-medium">Success</th>
                    <th className="px-8 py-3 font-medium">Failure</th>
                    <th className="px-8 py-3 font-medium">Avg Latency</th>
                    <th className="px-8 py-3 font-medium">Success Rate</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(aggregated.byLink).map(([id, row]) => (
                    <tr key={id} className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors">
                      <td className="px-8 py-4 text-sm font-semibold text-[var(--color-ink)]">{row.name}</td>
                      <td className="px-8 py-4 text-sm">{fmtNum(row.total)}</td>
                      <td className="px-8 py-4 text-sm text-[var(--color-success)]">{fmtNum(row.success)}</td>
                      <td className="px-8 py-4 text-sm text-[var(--color-error)]">{fmtNum(row.failure)}</td>
                      <td className="px-8 py-4 text-sm">{fmtMs(row.lat, row.total)}</td>
                      <td className="px-8 py-4 text-sm">
                        <Badge color={row.success / Math.max(row.total, 1) >= 0.95 ? 'success' : row.success / Math.max(row.total, 1) >= 0.8 ? 'warning' : 'error'}>
                          {fmtRate(row.success, row.total)}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Hourly detail table */}
          {stats.length > 0 && (
            <div className="overflow-hidden rounded-xl border border-[var(--color-hairline)] bg-[var(--color-canvas)]">
              <div className="px-8 py-5 border-b border-[var(--color-hairline-soft)]">
                <h3 className="text-lg font-semibold text-[var(--color-ink)]">Hourly Breakdown</h3>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-[var(--color-hairline-soft)] text-left text-sm font-medium text-[var(--color-muted)]">
                      <th className="px-8 py-3 font-medium">Period</th>
                      <th className="px-8 py-3 font-medium">Link</th>
                      <th className="px-8 py-3 font-medium">Total</th>
                      <th className="px-8 py-3 font-medium">Success</th>
                      <th className="px-8 py-3 font-medium">Failure</th>
                      <th className="px-8 py-3 font-medium">Avg Latency</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stats.filter((s) => s.AttrKey === '').slice(0, 100).map((s, i) => {
                      const link = links.find((l) => l.id === s.LinkID)
                      return (
                        <tr key={i} className="border-b border-[var(--color-hairline-soft)] last:border-b-0 hover:bg-[var(--color-surface-soft)] transition-colors">
                          <td className="px-8 py-3 text-sm whitespace-nowrap">{s.Period}</td>
                          <td className="px-8 py-3 text-sm font-semibold text-[var(--color-ink)]">{link?.name || s.LinkID}</td>
                          <td className="px-8 py-3 text-sm">{s.Total}</td>
                          <td className="px-8 py-3 text-sm text-[var(--color-success)]">{s.Success}</td>
                          <td className="px-8 py-3 text-sm text-[var(--color-error)]">{s.Failure}</td>
                          <td className="px-8 py-3 text-sm">{fmtMs(s.Lat, s.Total)}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
