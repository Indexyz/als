import assert from 'node:assert/strict'
import { test } from 'node:test'
import { appendTrafficSample, parseTrafficEvent } from './traffic.ts'

test('first sample only records counters', () => {
  const iface = appendTrafficSample(undefined, 'eth0', 100, 1000, 500)
  assert.deepEqual(iface, { name: 'eth0', totalRx: 1000, totalTx: 500, points: [] })
})

test('subsequent samples produce per-second deltas', () => {
  let iface = appendTrafficSample(undefined, 'eth0', 100, 1000, 500)
  iface = appendTrafficSample(iface, 'eth0', 101, 3000, 600)
  assert.deepEqual(iface.points, [{ time: 101000, rx: 2000, tx: 100 }])
  assert.equal(iface.totalRx, 3000)
})

test('counter resets never produce negative rates', () => {
  let iface = appendTrafficSample(undefined, 'eth0', 100, 5000, 5000)
  iface = appendTrafficSample(iface, 'eth0', 101, 10, 20)
  assert.deepEqual(iface.points[0], { time: 101000, rx: 0, tx: 0 })
})

test('duplicate or out-of-order timestamps do not add points', () => {
  let iface = appendTrafficSample(undefined, 'eth0', 100, 0, 0)
  iface = appendTrafficSample(iface, 'eth0', 101, 10, 10)
  iface = appendTrafficSample(iface, 'eth0', 101, 20, 20)
  assert.equal(iface.points.length, 1)
  assert.equal(iface.totalRx, 20)
})

test('keeps a bounded window of points', () => {
  let iface = appendTrafficSample(undefined, 'eth0', 0, 0, 0)
  for (let i = 1; i <= 10; i++) iface = appendTrafficSample(iface, 'eth0', i, i * 10, i, 4)
  assert.equal(iface.points.length, 4)
  assert.equal(iface.points[0]?.time, 7000)
})

test('parses SSE payloads and rejects malformed ones', () => {
  assert.deepEqual(parseTrafficEvent('eth0,100,1,2'), { name: 'eth0', time: 100, rx: 1, tx: 2 })
  assert.equal(parseTrafficEvent('eth0,100,x,2'), null)
  assert.equal(parseTrafficEvent(',100,1,2'), null)
  assert.equal(parseTrafficEvent('eth0'), null)
})
