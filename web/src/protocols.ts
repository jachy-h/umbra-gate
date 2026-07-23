import type { ProviderProtocol } from './types'

export const protocolOptions: { value: ProviderProtocol; label: string; shortLabel: string }[] = [
  { value: 'openai', label: 'OpenAI Style', shortLabel: 'OpenAI Style' },
  { value: 'anthropic', label: 'Anthropic Style', shortLabel: 'Anthropic Style' },
]

export const formatOptions = [
  { value: 'chat_completions', label: 'Chat Completions' },
  { value: 'responses', label: 'Responses' },
  { value: 'messages', label: 'Messages' },
] as const

export function formatLabel(format?: string) {
  return formatOptions.find((option) => option.value === format)?.label || format || 'Format not selected'
}

export function protocolLabel(protocol?: string) {
  return protocolOptions.find((option) => option.value === protocol)?.shortLabel || protocol || 'Protocol not selected'
}
