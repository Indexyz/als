/**
 * Throughput is sampled at a fixed cadence; a phase ends once a windowed estimate has held steady,
 * or at the cap. The reported rate is a trimmed mean of the last few seconds, so a link that bursts
 * and then throttles reports its sustained rate rather than a whole-run average. Everything here is
 * a pure function over sample arrays so it can be unit tested without a network.
 */

/** Seconds between throughput samples. */
export const SAMPLE_INTERVAL = 0.1
/** Seconds ignored at the start of a phase while TCP slow start ramps up. */
export const WARMUP = 1.5
export const MIN_DURATION = 7
export const MAX_DURATION = 15

/** Seconds of samples behind each stability estimate. */
const WINDOW = 3
/** Share of the window's samples dropped from each end before averaging. */
const TRIM = 0.1
/** Seconds of samples behind the reported rate. */
const RESULT_WINDOW = 5
/**
 * Shares of the result window dropped as stalls and bursts. The slow side is cut deeper because
 * request turnover leaves brief low samples that say nothing about the link.
 */
const RESULT_TRIM_SLOW = 0.2
const RESULT_TRIM_FAST = 0.1
/** Seconds of samples behind each point of the chart. */
const CHART_WINDOW = 1.5
/** Seconds the estimate must stay within TOLERANCE of its latest value. */
const STABLE_FOR = 2
const TOLERANCE = 0.04

export const toCount = (seconds: number): number => Math.round(seconds / SAMPLE_INTERVAL)

/** Mean with a share of the slowest and fastest samples dropped. */
export function trimmedMean(values: readonly number[], slow: number, fast: number): number {
  if (values.length === 0) return 0
  const sorted = [...values].sort((a, b) => a - b)
  const kept = sorted.slice(
    Math.floor(sorted.length * slow),
    sorted.length - Math.floor(sorted.length * fast)
  )
  if (kept.length === 0) return 0
  return kept.reduce((sum, value) => sum + value, 0) / kept.length
}

/** Symmetric trimmed mean of the post-warm-up samples in the trailing window; what the stop rule watches. */
export function estimate(samples: readonly number[]): number {
  const from = Math.max(toCount(WARMUP), samples.length - toCount(WINDOW))
  if (from >= samples.length) return samples.at(-1) ?? 0
  return trimmedMean(samples.slice(from), TRIM, TRIM)
}

/** The rate to report: the last few seconds with stalls and bursts discarded. Zero during warm-up. */
export function sustained(samples: readonly number[]): number {
  const from = Math.max(toCount(WARMUP), samples.length - toCount(RESULT_WINDOW))
  if (from >= samples.length) return 0
  return trimmedMean(samples.slice(from), RESULT_TRIM_SLOW, RESULT_TRIM_FAST)
}

/** The chart's view of the link: a short window minus request-turnover dips. */
export function smoothed(samples: readonly number[]): number {
  if (samples.length === 0) return 0
  return trimmedMean(samples.slice(-toCount(CHART_WINDOW)), RESULT_TRIM_SLOW, RESULT_TRIM_FAST)
}

/** One estimate is appended per sample, so its length is the elapsed time. */
export function shouldStop(estimates: readonly number[]): boolean {
  const count = estimates.length
  if (count >= toCount(MAX_DURATION)) return true
  if (count < toCount(MIN_DURATION)) return false
  const latest = estimates.at(-1) ?? 0
  if (latest <= 0) return false
  return estimates
    .slice(count - toCount(STABLE_FOR))
    .every((value) => Math.abs(value - latest) / latest < TOLERANCE)
}

export function median(values: readonly number[]): number | null {
  if (values.length === 0) return null
  const sorted = [...values].sort((a, b) => a - b)
  const mid = sorted.length >> 1
  return sorted.length % 2 === 0
    ? ((sorted[mid - 1] ?? 0) + (sorted[mid] ?? 0)) / 2
    : (sorted[mid] ?? 0)
}

/** Jitter as the mean absolute difference between consecutive round trips (RFC 3550 style). */
export function meanAbsoluteDelta(values: readonly number[]): number | null {
  if (values.length < 2) return null
  let sum = 0
  for (let i = 1; i < values.length; i++) sum += Math.abs((values[i] ?? 0) - (values[i - 1] ?? 0))
  return sum / (values.length - 1)
}
