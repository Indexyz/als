// Keep this contract aligned with backend/als/controller/diagnostics/target.go.

export type DiagnosticTool = 'ping' | 'traceroute' | 'mtr' | 'nexttrace'
export type DiagnosticFamily = 'auto' | '4' | '6'

export interface DiagnosticRow {
  seq?: number
  hop?: number
  host?: string
  ttl?: number | null
  latency_ms?: number | null
  timeout?: boolean
  loss_percent?: number
  sent?: number
  last_ms?: number | null
  avg_ms?: number | null
  best_ms?: number | null
  worst_ms?: number | null
  stddev_ms?: number | null
  rtts_ms?: number[]
  timeouts?: number
  asn?: string
  location?: string
  owner?: string
}

export type DiagnosticMessage =
  | { type: 'started'; address?: string }
  | { type: 'output'; line?: string }
  | { type: 'row'; row?: DiagnosticRow }
  | { type: 'snapshot'; rows?: DiagnosticRow[] }
  | { type: 'error'; code?: string }
  | { type: 'done'; status?: string }

export function isIPv4(value: string): boolean {
  if (/[^0-9.]/.test(value)) return false
  const parts = value.split('.')
  return (
    parts.length === 4 &&
    parts.every((part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255)
  )
}

export function isIPv6(value: string): boolean {
  if (!value.includes(':') || /[^0-9a-f:.]/i.test(value)) return false
  let address = value
  if (address.includes('.')) {
    const lastColon = address.lastIndexOf(':')
    if (!isIPv4(address.slice(lastColon + 1))) return false
    address = address.slice(0, lastColon + 1) + '0:0'
  }
  const halves = address.split('::')
  if (halves.length > 2) return false
  const groups = halves.flatMap((half) => (half === '' ? [] : half.split(':')))
  if (!groups.every((group) => /^[0-9a-f]{1,4}$/i.test(group))) return false
  return halves.length === 2 ? groups.length < 8 : groups.length === 8
}

export function isValidTarget(value: unknown): value is string {
  if (typeof value !== 'string' || !value || value.length > 254 || /[^\x21-\x7e]/.test(value))
    return false
  if (isIPv4(value) || isIPv6(value)) return true
  const name = value.endsWith('.') ? value.slice(0, -1) : value
  if (name.length > 253) return false
  const labels = name.split('.')
  return (
    labels.length >= 2 &&
    labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label)) &&
    /[a-z]/i.test(labels.at(-1) ?? '')
  )
}

export function diagnosticURL(location: string, sessionId: string): string {
  const url = new URL(`./session/${encodeURIComponent(sessionId)}/diagnostics`, location)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.search = ''
  url.hash = ''
  return url.href
}

export function rowKey(tool: string, row: DiagnosticRow): string {
  if (tool === 'ping') return `ping:${row.seq}`
  return `${row.hop}:${row.host}`
}

export function updateDiagnosticRows(
  current: DiagnosticRow[],
  tool: string,
  message: DiagnosticMessage
): DiagnosticRow[] {
  let rows: DiagnosticRow[]
  if (message.type === 'snapshot') {
    rows = Array.isArray(message.rows) ? [...message.rows] : []
  } else if (message.type === 'row' && message.row) {
    rows = [...current]
    const key = rowKey(tool, message.row)
    const index = rows.findIndex((entry) => rowKey(tool, entry) === key)
    if (index < 0) rows.push(message.row)
    else rows[index] = message.row
  } else {
    return current
  }
  return rows.sort((a, b) => (a.seq ?? a.hop ?? 0) - (b.seq ?? b.hop ?? 0))
}

export function formatMilliseconds(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(2)} ms` : '—'
}

export type LatencyTone = 'good' | 'fair' | 'poor' | 'none'

/** Classify a round-trip time for colour coding: <80 ms good, <200 ms fair, otherwise poor. */
export function latencyTone(value: unknown): LatencyTone {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 'none'
  if (value < 80) return 'good'
  if (value < 200) return 'fair'
  return 'poor'
}

/** Classify packet loss: 0 % good, <20 % fair, otherwise poor. */
export function lossTone(value: unknown): LatencyTone {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 'none'
  if (value <= 0) return 'good'
  if (value < 20) return 'fair'
  return 'poor'
}

export interface PingSummary {
  sent: number
  received: number
  loss: number
  min: number | null
  avg: number | null
  max: number | null
}

export function summarizePing(rows: DiagnosticRow[]): PingSummary {
  const latencies = rows
    .filter((row) => !row.timeout)
    .map((row) => row.latency_ms)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  const sent = rows.length
  const received = latencies.length
  return {
    sent,
    received,
    loss: sent ? ((sent - received) / sent) * 100 : 0,
    min: received ? Math.min(...latencies) : null,
    avg: received ? latencies.reduce((sum, value) => sum + value, 0) / received : null,
    max: received ? Math.max(...latencies) : null
  }
}
