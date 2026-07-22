const BASE = '/admin'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  getTypes: () => request<{ types: string[] }>('/types'),

  listProviders: async () => (await request<import('./types').Provider[] | null>('/providers')) ?? [],
  createProvider: (p: Partial<import('./types').Provider>) =>
    request<import('./types').Provider>('/providers', { method: 'POST', body: JSON.stringify(p) }),
  getProvider: (id: string) => request<import('./types').Provider>(`/providers/${id}`),
  deleteProvider: (id: string) => request<void>(`/providers/${id}`, { method: 'DELETE' }),

  listLinks: async () => (await request<import('./types').ProxyLink[] | null>('/links')) ?? [],
  createLink: (l: Partial<import('./types').ProxyLink>) =>
    request<import('./types').ProxyLink>('/links', { method: 'POST', body: JSON.stringify(l) }),
  getLink: (id: string) => request<import('./types').ProxyLink>(`/links/${id}`),
  deleteLink: (id: string) => request<void>(`/links/${id}`, { method: 'DELETE' }),

  getStats: (params?: { link_id?: string; from?: string; to?: string }) => {
    const qs = new URLSearchParams()
    if (params?.link_id) qs.set('link_id', params.link_id)
    if (params?.from) qs.set('from', params.from)
    if (params?.to) qs.set('to', params.to)
    const q = qs.toString()
    return request<import('./types').StatsResponse>(`/stats${q ? `?${q}` : ''}`)
  },
}
