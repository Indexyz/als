import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  MAX_DURATION,
  MIN_DURATION,
  estimate,
  meanAbsoluteDelta,
  median,
  shouldStop,
  smoothed,
  sustained,
  toCount,
  trimmedMean
} from './stability.ts'

test('trimmed mean drops the requested shares from each end', () => {
  assert.equal(trimmedMean([], 0.1, 0.1), 0)
  assert.equal(trimmedMean([1, 2, 3, 4, 5, 6, 7, 8, 9, 100], 0.1, 0.1), 5.5)
  assert.equal(trimmedMean([0, 10, 10, 10, 10], 0.2, 0), 10)
})

test('sustained rate ignores warm-up and is zero until it has passed', () => {
  const warmup = Array.from({ length: toCount(1.5) }, () => 1000)
  assert.equal(sustained(warmup), 0)
  const steady = [...warmup, ...Array.from({ length: 50 }, () => 100)]
  assert.equal(sustained(steady), 100)
  // A burst then throttle reports the throttled rate.
  const throttled = [
    ...warmup,
    ...Array.from({ length: 30 }, () => 500),
    ...Array.from({ length: 50 }, () => 50)
  ]
  assert.equal(sustained(throttled), 50)
})

test('estimate falls back to the last sample during warm-up', () => {
  assert.equal(estimate([]), 0)
  assert.equal(estimate([3, 7]), 7)
})

test('smoothed tolerates turnover dips', () => {
  const samples = Array.from({ length: 15 }, (_, i) => (i % 5 === 0 ? 0 : 100))
  assert.ok(smoothed(samples) > 90)
  assert.equal(smoothed([]), 0)
})

test('stop rule: never before the minimum, always at the cap, otherwise when stable', () => {
  const stable = Array.from({ length: toCount(MIN_DURATION) - 1 }, () => 100)
  assert.equal(shouldStop(stable), false)
  assert.equal(shouldStop([...stable, 100]), true)
  const noisy = Array.from({ length: toCount(MIN_DURATION) }, (_, i) => (i % 2 ? 50 : 150))
  assert.equal(shouldStop(noisy), false)
  assert.equal(shouldStop(Array.from({ length: toCount(MAX_DURATION) }, (_, i) => i % 2)), true)
  assert.equal(shouldStop(Array.from({ length: toCount(MIN_DURATION) }, () => 0)), false)
})

test('median and jitter', () => {
  assert.equal(median([]), null)
  assert.equal(median([5, 1, 3]), 3)
  assert.equal(median([4, 1, 3, 2]), 2.5)
  assert.equal(meanAbsoluteDelta([10]), null)
  assert.equal(meanAbsoluteDelta([10, 12, 9, 9]), (2 + 3 + 0) / 3)
})
