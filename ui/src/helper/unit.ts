const SIZE_UNITS = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'] as const
const BANDWIDTH_UNITS = [
  'bps',
  'Kbps',
  'Mbps',
  'Gbps',
  'Tbps',
  'Pbps',
  'Ebps',
  'Zbps',
  'Ybps'
] as const

/**
 * Format a byte count (base 1024), or with `bandwidth` a bytes-per-second rate as bits per
 * second (base 1000).
 */
export function formatBytes(input: number | string, decimals = 2, bandwidth = false): string {
  const value = Number(input)
  const units = bandwidth ? BANDWIDTH_UNITS : SIZE_UNITS
  if (!Number.isFinite(value) || value <= 0) return `0 ${units[0]}`

  const base = bandwidth ? 1000 : 1024
  const scaled = bandwidth ? value * 8 : value
  const index = Math.min(
    units.length - 1,
    Math.max(0, Math.floor(Math.log(scaled) / Math.log(base)))
  )
  const dm = decimals < 0 ? 0 : decimals
  return `${parseFloat((scaled / Math.pow(base, index)).toFixed(dm))} ${units[index]}`
}

/** Split a formatted value into its number and unit, for large-number displays. */
export function splitUnit(formatted: string): { value: string; unit: string } {
  const [value = formatted, unit = ''] = formatted.split(' ')
  return { value, unit }
}
