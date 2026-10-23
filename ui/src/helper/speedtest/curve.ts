export interface Point {
  x: number
  y: number
}

/** Catmull-Rom spline through the points as cubic Béziers, so the line bends instead of cornering. */
export function curvePath(points: readonly Point[]): string {
  const first = points[0]
  if (!first) return ''
  let d = `M${first.x},${first.y}`
  const last = points.length - 1
  for (let i = 0; i < last; i++) {
    const before = points[Math.max(0, i - 1)] ?? first
    const from = points[i] ?? first
    const to = points[i + 1] ?? from
    const after = points[Math.min(last, i + 2)] ?? to
    d +=
      ` C${from.x + (to.x - before.x) / 6},${from.y + (to.y - before.y) / 6}` +
      ` ${to.x - (after.x - from.x) / 6},${to.y - (after.y - from.y) / 6}` +
      ` ${to.x},${to.y}`
  }
  return d
}

export interface TrendGeometry {
  line: string
  area: string
  markerY: number | null
}

/**
 * Lay out a left-to-right trend over `capacity` slots in a WIDTH x HEIGHT box, normalised to the
 * running peak with headroom, sampling every `stride`th point (always including the last).
 */
export function trendGeometry(
  samples: readonly number[],
  capacity: number,
  marker: number | null,
  width: number,
  height: number,
  stride = 2
): TrendGeometry {
  const peak = Math.max(1, marker ?? 0, ...samples)
  const usable = height * 0.9
  const slots = Math.max(2, capacity)
  const pointAt = (i: number): Point => ({
    x: (i / (slots - 1)) * width,
    y: height - ((samples[i] ?? 0) / peak) * usable
  })
  const points: Point[] = []
  for (let i = 0; i < samples.length; i += stride) points.push(pointAt(i))
  if (samples.length > 1 && (samples.length - 1) % stride !== 0)
    points.push(pointAt(samples.length - 1))
  const line = points.length > 1 ? curvePath(points) : ''
  const lastX = points.at(-1)?.x ?? 0
  const area = line ? `${line} L${lastX},${height} L0,${height} Z` : ''
  const markerY = marker !== null && marker > 0 ? height - (marker / peak) * usable : null
  return { line, area, markerY }
}
