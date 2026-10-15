import type { JsonObject, JsonValue } from '@bufbuild/protobuf'

function show(v: JsonValue | undefined): string {
  if (v === null || v === undefined || v === '') return '—'
  if (Array.isArray(v)) return v.map(show).join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

export function summarize(details: JsonObject | undefined): string {
  if (!details) return ''
  const parts: string[] = []
  if ('from' in details || 'to' in details)
    parts.push(`${show(details.from)} → ${show(details.to)}`)
  for (const [key, value] of Object.entries(details)) {
    if (key === 'from' || key === 'to') continue
    if (value === true) parts.push(key)
    else parts.push(`${key}: ${show(value)}`)
  }
  return parts.join(' · ')
}
