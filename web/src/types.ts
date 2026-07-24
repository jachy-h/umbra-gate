export type ProviderProtocol = 'openai' | 'anthropic'
export type EndpointFormat = 'chat_completions' | 'responses' | 'messages'

export interface ProviderEndpoint {
  protocol: ProviderProtocol
  request_format: EndpointFormat
  response_format: EndpointFormat
  base_url: string
}

export interface Provider {
  id: string
  name: string
  type: string
  base_url: string
  endpoints: ProviderEndpoint[]
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
  protocol: ProviderProtocol | ''
  retry_count: number
  fallback_model: string
  api_key?: string
  rules?: Rules
  validation_ok?: boolean
	validation_error?: string
	validated_at?: string
	supported_formats?: EndpointFormat[]
}

export interface RequestLog {
  id: string
  link_id: string
  path: string
  provider_id: string
  provider_name: string
  model: string
  status_code: number
  latency_ms: number
  success: boolean
  error_message?: string
  request_url?: string
  request_headers?: Record<string, unknown>
  request_body?: string
  upstream_url?: string
  upstream_headers?: Record<string, unknown>
  upstream_body?: string
  response_headers?: Record<string, unknown>
  response_body?: string
  attributes?: Record<string, unknown>
  created_at: string
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
	protocol: ProviderProtocol | ''
	supported_formats?: EndpointFormat[]
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
