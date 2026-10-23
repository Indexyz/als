/**
 * A run measures idle latency and jitter, then download and upload throughput. While each
 * throughput phase saturates the link, a ping runs every LOADED_PING_INTERVAL to measure loaded
 * latency (bufferbloat). Single-thread mode keeps exactly one transfer in flight per direction,
 * multi-thread mode several concurrent ones. ALS is usually served over HTTP/1.1,
 * where each in-flight request is its own TCP connection; so the thread count maps directly to connections.
 */
import {
  SAMPLE_INTERVAL,
  estimate,
  meanAbsoluteDelta,
  median,
  shouldStop,
  smoothed,
  sustained
} from './stability.ts'

export type ThreadMode = 'single' | 'multi'
export type Phase = 'idle' | 'latency' | 'download' | 'upload' | 'done'

/** One throughput phase. Each figure is live while the phase runs and becomes the result when it ends. */
export interface Throughput {
  /** Mbps: the chart's current value while running, then the result. */
  speed: number | null
  /** Mbps the result is heading for; equals the result once the phase ends. */
  sustained: number | null
  /** Median loaded latency in ms; null until the first loaded ping lands. */
  latency: number | null
  /** Loaded jitter in ms. */
  jitter: number | null
  /** Chart readings in Mbps, one per sample interval. */
  trend: number[]
  /** Bytes transferred so far. */
  bytes: number
  /** Seconds elapsed in this phase. */
  elapsed: number
}

export const NOT_STARTED: Throughput = {
  speed: null,
  sustained: null,
  latency: null,
  jitter: null,
  trend: [],
  bytes: 0,
  elapsed: 0
}

export interface Report {
  phase: (phase: Phase) => void
  /** Idle latency: each ping while the latency phase runs, then the median. */
  latency: (ms: number | null) => void
  jitter: (ms: number | null) => void
  download: (throughput: Throughput) => void
  upload: (throughput: Throughput) => void
}

/** Network and clock access, injectable for tests. */
export interface Transport {
  now: () => number
  wait: (ms: number, signal?: AbortSignal) => Promise<void>
  /** Round trip in ms. */
  ping: (signal: AbortSignal) => Promise<number>
  /** Stream one download body, calling onChunk per received chunk; resolves when it ends. */
  download: (signal: AbortSignal, onChunk: (bytes: number) => void) => Promise<void>
  /** Send `bytes` and resolve once the server acknowledged them. */
  upload: (bytes: number, signal: AbortSignal) => Promise<void>
}

export interface EngineOptions {
  mode: ThreadMode
  /** Concurrent connections per direction in multi-thread mode. */
  streams?: number
  /** Skip the upload phase. */
  skipUpload?: boolean
}

export const MULTI_STREAMS = 4
/** MiB per download request. */
const DOWNLOAD_MIB = 25
// Upload bodies start small and grow so each takes about UPLOAD_TARGET seconds. Each POST is timed
// to the server's reply: the only proof the bytes left, as proxies accept bodies at memory speed.
export const UPLOAD_FIRST_BYTES = 256 * 1024
export const UPLOAD_MAX_BYTES = 32 * 1024 * 1024
const UPLOAD_TARGET = 1
const LATENCY_PINGS_MIN = 8
const LATENCY_PINGS_MAX = 20
const LATENCY_BUDGET = 2000
const LOADED_PING_INTERVAL = 400
/** Most recent loaded pings that form the loaded-latency result. */
const LOADED_POINTS = 20
const RETRY_DELAY = 300

const CHART_POLISH = 1 - Math.exp(-SAMPLE_INTERVAL / 0.3)
const SMOOTHING = 1 - Math.exp(-SAMPLE_INTERVAL / 0.6)
const smooth = (shown: number, reading: number) => shown + (reading - shown) * SMOOTHING

export const browserWait = (ms: number, signal?: AbortSignal): Promise<void> =>
  new Promise((resolve) => {
    if (signal?.aborted) return resolve()
    const timer = setTimeout(done, ms)
    function done() {
      clearTimeout(timer)
      signal?.removeEventListener('abort', done)
      resolve()
    }
    signal?.addEventListener('abort', done, { once: true })
  })

/** Transport against this server's session-scoped speed test endpoints. */
export function createHttpTransport(baseUrl: string): Transport {
  const init = { cache: 'no-store', referrerPolicy: 'no-referrer' } as const satisfies RequestInit
  const bust = () => `r=${Math.random().toString(36).slice(2)}`
  return {
    now: () => performance.now(),
    wait: browserWait,
    async ping(signal) {
      const url = `${baseUrl}/ping?${bust()}`
      const started = performance.now()
      const response = await fetch(url, { ...init, method: 'HEAD', signal })
      const headersAt = performance.now()
      if (!response.ok) throw new Error(`Ping failed: ${response.status}`)
      const entry = performance.getEntriesByName(url, 'resource').at(-1) as
        PerformanceResourceTiming | undefined
      // Resource Timing excludes queueing and connection setup when available.
      return entry && entry.requestStart > 0 && entry.responseStart > 0
        ? entry.responseStart - entry.requestStart
        : headersAt - started
    },
    async download(signal, onChunk) {
      const response = await fetch(`${baseUrl}/download?ckSize=${DOWNLOAD_MIB}&${bust()}`, {
        ...init,
        signal
      })
      if (!response.ok || !response.body) throw new Error(`Download failed: ${response.status}`)
      const reader = response.body.getReader()
      try {
        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          onChunk(value.byteLength)
        }
      } finally {
        reader.releaseLock()
      }
    },
    async upload(bytes, signal) {
      const response = await fetch(`${baseUrl}/upload?${bust()}`, {
        ...init,
        method: 'POST',
        body: new Blob([new Uint8Array(bytes)]),
        signal
      })
      await response.body?.cancel()
      if (!response.ok) throw new Error(`Upload rejected: ${response.status}`)
    }
  }
}

interface Meter {
  /** Bytes that just streamed through; the sampler turns them into a rate. */
  flow: (count: number) => void
  /** A request of `bytes` acknowledged after `seconds`; its rate holds until the next one. */
  transfer: (bytes: number, seconds: number) => void
}

type Pump = (signal: AbortSignal, meter: Meter) => Promise<void>

/**
 * Keeps one download connection busy with back-to-back requests. There is deliberately no
 * prefetching: over HTTP/1.1 an overlapping request opens another connection, which would make
 * single-thread mode multi-connection. Bodies are large enough that the gap is negligible.
 */
function downloadPump(transport: Transport): Pump {
  return async (signal, meter) => {
    while (!signal.aborted) {
      try {
        await transport.download(signal, meter.flow)
      } catch {
        if (signal.aborted) break
        await transport.wait(RETRY_DELAY, signal)
      }
    }
  }
}

/** Sequential POSTs on one connection, each timed to the server's reply. */
function uploadPump(transport: Transport): Pump {
  return async (signal, meter) => {
    let bytes = UPLOAD_FIRST_BYTES
    while (!signal.aborted) {
      try {
        const started = transport.now()
        await transport.upload(bytes, signal)
        const seconds = Math.max(0.001, (transport.now() - started) / 1000)
        meter.transfer(bytes, seconds)
        bytes = nextUploadSize(bytes, seconds)
      } catch {
        if (signal.aborted) break
        meter.transfer(0, 1)
        await transport.wait(RETRY_DELAY, signal)
      }
    }
  }
}

/** Grow or shrink the next upload body so it takes about UPLOAD_TARGET seconds. */
export function nextUploadSize(bytes: number, seconds: number): number {
  return Math.min(
    UPLOAD_MAX_BYTES,
    Math.max(UPLOAD_FIRST_BYTES, Math.round((bytes / seconds) * UPLOAD_TARGET))
  )
}

/** Runs the pumps until the stability rule fires, sampling throughput and pinging under load. */
async function measure(
  transport: Transport,
  pumps: readonly Pump[],
  signal: AbortSignal,
  onProgress: (reading: Throughput) => void
): Promise<void> {
  const phase = new AbortController()
  const stop = () => phase.abort()
  signal.addEventListener('abort', stop, { once: true })

  let pending = 0
  let acknowledged = 0
  const held = pumps.map(() => 0)
  const streams = pumps.map((start, i) =>
    start(phase.signal, {
      flow(count) {
        pending += count
      },
      transfer(bytes, seconds) {
        acknowledged += bytes
        held[i] = (bytes * 8) / seconds / 1e6
      }
    })
  )

  const loadedPings: number[] = []
  let shownLoaded: number | null = null
  const pinger = (async () => {
    for (;;) {
      await transport.wait(LOADED_PING_INTERVAL, phase.signal)
      if (phase.signal.aborted) return
      try {
        const ms = await transport.ping(phase.signal)
        loadedPings.push(ms)
        shownLoaded = shownLoaded === null ? ms : smooth(shownLoaded, ms)
      } catch {
        // Aborted or dropped; the next interval tries again.
      }
    }
  })()

  const samples: number[] = []
  const estimates: number[] = []
  const trend: number[] = []
  let chartLine = 0
  let marker = 0
  let bytes = 0
  const started = transport.now()
  const loaded = () => loadedPings.slice(-LOADED_POINTS)
  const report = (speed: number, mark: number | null, latency: number | null) => {
    if (signal.aborted) return
    onProgress({
      speed,
      sustained: mark,
      latency,
      jitter: meanAbsoluteDelta(loaded()),
      trend: [...trend],
      bytes,
      elapsed: (transport.now() - started) / 1000
    })
  }

  let last = transport.now()
  for (;;) {
    await transport.wait(SAMPLE_INTERVAL * 1000, phase.signal)
    if (phase.signal.aborted) break
    const now = transport.now()
    const seconds = Math.max(0.001, (now - last) / 1000)
    let mbps = (pending * 8) / seconds / 1e6
    for (const rate of held) mbps += rate
    bytes += pending + acknowledged
    pending = 0
    acknowledged = 0
    last = now
    samples.push(mbps)
    estimates.push(estimate(samples))
    chartLine += (smoothed(samples) - chartLine) * CHART_POLISH
    trend.push(chartLine)
    const rate = sustained(samples)
    marker = marker === 0 ? rate : smooth(marker, rate)
    report(chartLine, marker > 0 ? marker : null, shownLoaded)
    if (shouldStop(estimates)) break
  }

  stop()
  signal.removeEventListener('abort', stop)
  await Promise.allSettled([...streams, pinger])
  const result = sustained(samples)
  report(result, result, median(loaded()))
}

/**
 * Runs the full test, reporting each figure as it changes. Rejects with the abort reason when the
 * signal fires mid-run, or with the ping error when the server is unreachable.
 */
export async function runSpeedtest(
  transport: Transport,
  options: EngineOptions,
  signal: AbortSignal,
  report: Report
): Promise<void> {
  const streams = options.mode === 'multi' ? (options.streams ?? MULTI_STREAMS) : 1

  report.phase('latency')
  const pings: number[] = []
  // The first ping warms up the connection and is discarded.
  await transport.ping(signal)
  const budgetEnds = transport.now() + LATENCY_BUDGET
  while (
    pings.length < LATENCY_PINGS_MAX &&
    (pings.length < LATENCY_PINGS_MIN || transport.now() < budgetEnds)
  ) {
    const ms = await transport.ping(signal)
    pings.push(ms)
    report.latency(ms)
  }
  report.latency(median(pings))
  report.jitter(meanAbsoluteDelta(pings))
  signal.throwIfAborted()

  report.phase('download')
  await measure(
    transport,
    Array.from({ length: streams }, () => downloadPump(transport)),
    signal,
    report.download
  )
  signal.throwIfAborted()

  if (!options.skipUpload) {
    report.phase('upload')
    await measure(
      transport,
      Array.from({ length: streams }, () => uploadPump(transport)),
      signal,
      report.upload
    )
    signal.throwIfAborted()
  }
  report.phase('done')
}
