export const MAX_TRAFFIC_POINTS = 60

export interface TrafficPoint {
  /** Unix time in milliseconds. */
  time: number
  /** Bytes per second. */
  rx: number
  tx: number
}

export interface InterfaceTraffic {
  name: string
  /** Cumulative counters as last reported by the server. */
  totalRx: number
  totalTx: number
  points: TrafficPoint[]
}

/**
 * Add a cumulative counter sample (`time` in seconds) and derive the per-second rate from the
 * previous sample. Counter resets (e.g. interface restart) are treated as zero throughput.
 */
export function appendTrafficSample(
  current: InterfaceTraffic | undefined,
  name: string,
  time: number,
  rx: number,
  tx: number,
  maxPoints = MAX_TRAFFIC_POINTS
): InterfaceTraffic {
  if (!current) return { name, totalRx: rx, totalTx: tx, points: [] }
  const last = current.points.at(-1)
  const timeMs = time * 1000
  if (last && timeMs <= last.time) {
    return { ...current, totalRx: rx, totalTx: tx }
  }
  const point: TrafficPoint = {
    time: timeMs,
    rx: Math.max(0, rx - current.totalRx),
    tx: Math.max(0, tx - current.totalTx)
  }
  const points = [...current.points, point]
  return {
    name,
    totalRx: rx,
    totalTx: tx,
    points: points.length > maxPoints ? points.slice(-maxPoints) : points
  }
}

/** Parse the `InterfaceTraffic` SSE payload: `name,unixSeconds,rxBytes,txBytes`. */
export function parseTrafficEvent(
  data: string
): { name: string; time: number; rx: number; tx: number } | null {
  const [name, time, rx, tx] = data.split(',')
  const values = [time, rx, tx].map(Number)
  if (!name || values.some((value) => !Number.isFinite(value))) return null
  const [t = 0, r = 0, x = 0] = values
  return { name, time: t, rx: r, tx: x }
}
