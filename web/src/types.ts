export interface Provider {
  id: string
  name: string
  type: string
  base_url: string
  api_key?: string
  models: string[]
  extra?: Record<string, unknown>
  enabled: boolean
  builtin: boolean
  has_api_key: boolean
  created_at: string
}

export interface ChainEntry {
  provider_id: string
  retry_count: number
  fallback_model: string
  api_key?: string
  rules?: Rules
}

export interface Rules {
  on_status_codes?: number[]
  on_errors?: string[]
  on_timeout?: boolean
}

export interface ProxyLink {
  id: string
  name: string
  path: string
  attributes: Record<string, unknown>
  chain: ChainEntry[]
  enabled: boolean
  created_at: string
}

export interface StatsRow {
  LinkID: string
  ProviderID: string
  AttrKey: string
  AttrValue: string
  Period: string
  Total: number
  Success: number
  Failure: number
  Lat: number
}

export interface StatsResponse {
  stats: StatsRow[]
}

export interface TypesResponse {
  types: string[]
}
