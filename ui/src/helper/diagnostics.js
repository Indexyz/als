// Keep this contract aligned with backend/als/controller/diagnostics/target.go.
export function isIPv4(value) {
  if (/[^0-9.]/.test(value)) return false
  const parts = value.split('.')
  return (
    parts.length === 4 &&
    parts.every((part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255)
  )
}

export function isIPv6(value) {
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

export function isValidTarget(value) {
  if (typeof value !== 'string' || !value || value.length > 254 || /[^\x21-\x7e]/.test(value))
    return false
  if (isIPv4(value) || isIPv6(value)) return true
  const name = value.endsWith('.') ? value.slice(0, -1) : value
  if (name.length > 253) return false
  const labels = name.split('.')
  return (
    labels.length >= 2 &&
    labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label)) &&
    /[a-z]/i.test(labels.at(-1))
  )
}

export function diagnosticURL(location, sessionId) {
  const url = new URL(`./session/${encodeURIComponent(sessionId)}/diagnostics`, location)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.search = ''
  url.hash = ''
  return url.href
}

export function rowKey(tool, row) {
  if (tool === 'ping') return `ping:${row.seq}`
  return `${row.hop}:${row.host}`
}

export function updateDiagnosticRows(current, tool, message) {
  let rows
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
  return rows.sort((a, b) => (a.seq ?? a.hop) - (b.seq ?? b.hop))
}

export function formatMilliseconds(value) {
  return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(2)} ms` : '—'
}
