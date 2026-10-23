import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  UPLOAD_FIRST_BYTES,
  UPLOAD_MAX_BYTES,
  nextUploadSize,
  runSpeedtest,
  type Phase,
  type Throughput,
  type Transport
} from './engine.ts'
import { MAX_DURATION } from './stability.ts'

/**
 * A simulated link with a virtual clock. Each connection gets `perStreamMbps`; idle pings take
 * `idleMs`, pings during a transfer take `loadedMs`.
 */
function simulatedLink(options: {
  perStreamMbps: number
  idleMs: number
  loadedMs: number
  failPing?: boolean
}) {
  let clock = 0
  let active = 0
  let maxActive = 0
  const timers: { at: number; resolve: () => void }[] = []
  const wait = (ms: number, signal?: AbortSignal) =>
    new Promise<void>((resolve) => {
      if (signal?.aborted) return resolve()
      const timer = { at: clock + ms, resolve }
      timers.push(timer)
      signal?.addEventListener(
        'abort',
        () => {
          timers.splice(timers.indexOf(timer), 1)
          resolve()
        },
        { once: true }
      )
    })
  const abortable = (ms: number, signal: AbortSignal) =>
    wait(ms, signal).then(() => signal.throwIfAborted())
  const bytesPerMs = (options.perStreamMbps * 1e6) / 8 / 1000

  const transport: Transport = {
    now: () => clock,
    wait,
    async ping(signal) {
      if (options.failPing) throw new Error('unreachable')
      const ms = active > 0 ? options.loadedMs : options.idleMs
      await abortable(ms, signal)
      return ms
    },
    async download(signal, onChunk) {
      const total = 4 * 1024 * 1024
      active++
      maxActive = Math.max(maxActive, active)
      try {
        const chunk = 64 * 1024
        for (let sent = 0; sent < total; sent += chunk) {
          await abortable(chunk / bytesPerMs, signal)
          onChunk(chunk)
        }
      } finally {
        active--
      }
    },
    async upload(bytes, signal) {
      active++
      maxActive = Math.max(maxActive, active)
      try {
        await abortable(bytes / bytesPerMs, signal)
      } finally {
        active--
      }
    }
  }

  /** Advance virtual time until `done` settles. */
  async function drive<T>(done: Promise<T>): Promise<T> {
    let settled = false
    done.then(
      () => (settled = true),
      () => (settled = true)
    )
    while (!settled) {
      await new Promise((resolve) => setImmediate(resolve))
      if (settled) break
      timers.sort((a, b) => a.at - b.at)
      const next = timers.shift()
      if (!next) continue
      clock = Math.max(clock, next.at)
      next.resolve()
    }
    return done
  }

  return { transport, drive, maxActive: () => maxActive }
}

function collect() {
  const phases: Phase[] = []
  const latencies: (number | null)[] = []
  const jitters: (number | null)[] = []
  let download: Throughput | null = null
  let upload: Throughput | null = null
  return {
    phases,
    latencies,
    jitters,
    get download() {
      return download
    },
    get upload() {
      return upload
    },
    report: {
      phase: (phase: Phase) => phases.push(phase),
      latency: (ms: number | null) => latencies.push(ms),
      jitter: (ms: number | null) => jitters.push(ms),
      download: (value: Throughput) => (download = value),
      upload: (value: Throughput) => (upload = value)
    }
  }
}

test('single-thread run reports idle and loaded latency, speed and bytes', async () => {
  const link = simulatedLink({ perStreamMbps: 100, idleMs: 10, loadedMs: 60 })
  const result = collect()
  await link.drive(
    runSpeedtest(link.transport, { mode: 'single' }, new AbortController().signal, result.report)
  )
  assert.deepEqual(result.phases, ['latency', 'download', 'upload', 'done'])
  assert.equal(result.latencies.at(-1), 10)
  assert.equal(result.jitters.at(-1), 0)
  assert.equal(link.maxActive(), 1)

  for (const phase of [result.download, result.upload]) {
    assert.ok(phase)
    assert.ok(Math.abs((phase.speed ?? 0) - 100) < 10, `speed ${phase.speed}`)
    assert.equal(phase.speed, phase.sustained)
    assert.equal(phase.latency, 60)
    assert.equal(phase.jitter, 0)
    assert.ok(phase.bytes > 50e6, `bytes ${phase.bytes}`)
    assert.ok(phase.elapsed >= 7 && phase.elapsed <= MAX_DURATION + 1, `elapsed ${phase.elapsed}`)
    assert.ok(phase.trend.length > 50)
  }
})

test('multi-thread run uses several connections and scales throughput', async () => {
  const link = simulatedLink({ perStreamMbps: 50, idleMs: 5, loadedMs: 20 })
  const result = collect()
  await link.drive(
    runSpeedtest(
      link.transport,
      { mode: 'multi', streams: 4, skipUpload: true },
      new AbortController().signal,
      result.report
    )
  )
  assert.deepEqual(result.phases, ['latency', 'download', 'done'])
  assert.ok(link.maxActive() >= 4, `max active ${link.maxActive()}`)
  assert.ok(Math.abs((result.download?.speed ?? 0) - 200) < 20, `${result.download?.speed}`)
  assert.equal(result.upload, null)
})

test('aborting mid-download rejects and stops reporting', async () => {
  const link = simulatedLink({ perStreamMbps: 100, idleMs: 10, loadedMs: 30 })
  const result = collect()
  const controller = new AbortController()
  const run = runSpeedtest(link.transport, { mode: 'single' }, controller.signal, {
    ...result.report,
    download: (value) => {
      result.report.download(value)
      if (value.elapsed > 2) controller.abort()
    }
  })
  await assert.rejects(link.drive(run))
  assert.ok(!result.phases.includes('upload'))
  assert.ok(!result.phases.includes('done'))
})

test('unreachable server fails during the latency phase', async () => {
  const link = simulatedLink({ perStreamMbps: 100, idleMs: 10, loadedMs: 30, failPing: true })
  const result = collect()
  await assert.rejects(
    link.drive(
      runSpeedtest(link.transport, { mode: 'single' }, new AbortController().signal, result.report)
    ),
    /unreachable/
  )
  assert.deepEqual(result.phases, ['latency'])
})

test('upload body size adapts toward one second per request within bounds', () => {
  assert.equal(nextUploadSize(UPLOAD_FIRST_BYTES, 10), UPLOAD_FIRST_BYTES)
  assert.equal(nextUploadSize(1_000_000, 0.5), 2_000_000)
  assert.equal(nextUploadSize(UPLOAD_MAX_BYTES, 0.01), UPLOAD_MAX_BYTES)
})
