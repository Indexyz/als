import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  diagnosticURL,
  formatMilliseconds,
  isValidTarget,
  rowKey,
  updateDiagnosticRows
} from './diagnostics.js'

const targets = JSON.parse(
  readFileSync(new URL('./diagnostic-targets.json', import.meta.url), 'utf8')
)
for (const [kind, values] of Object.entries(targets)) {
  for (const target of values) {
    test(`${kind} target: ${JSON.stringify(target)}`, () =>
      assert.equal(isValidTarget(target), kind === 'valid'))
  }
}
test('DNS label and total length boundaries', () => {
  assert.equal(isValidTarget(`${'a'.repeat(63)}.example`), true)
  assert.equal(isValidTarget(`${'a'.repeat(64)}.example`), false)
  const max = ['a'.repeat(63), 'b'.repeat(63), 'c'.repeat(63), 'd'.repeat(61)].join('.')
  assert.equal(max.length, 253)
  assert.equal(isValidTarget(max), true)
  assert.equal(isValidTarget(max + '.'), true)
  assert.equal(isValidTarget(max + 'a'), false)
  assert.equal(isValidTarget(null), false)
})
test('WebSocket URLs preserve reverse proxy prefixes and use TLS', () => {
  assert.equal(
    diagnosticURL('https://lg.example/looking-glass/?lang=en#tools', 'abc'),
    'wss://lg.example/looking-glass/session/abc/diagnostics'
  )
  assert.equal(
    diagnosticURL('http://localhost:8080/', 'abc'),
    'ws://localhost:8080/session/abc/diagnostics'
  )
})
test('MTR snapshots remove stale timeouts and shortened paths', () => {
  const previous = [
    { hop: 1, host: '*' },
    { hop: 2, host: '192.0.2.2' }
  ]
  const next = updateDiagnosticRows(previous, 'mtr', {
    type: 'snapshot',
    rows: [{ hop: 1, host: '192.0.2.1', sent: 2 }]
  })
  assert.deepEqual(next, [{ hop: 1, host: '192.0.2.1', sent: 2 }])
  assert.equal(previous.length, 2)
  assert.deepEqual(updateDiagnosticRows(next, 'mtr', { type: 'snapshot' }), [])
})
test('Multipath trace rows do not overwrite other responders', () => {
  let rows = updateDiagnosticRows([], 'traceroute', {
    type: 'row',
    row: { hop: 2, host: '192.0.2.1', rtts_ms: [1] }
  })
  rows = updateDiagnosticRows(rows, 'traceroute', {
    type: 'row',
    row: { hop: 2, host: '192.0.2.2', rtts_ms: [2] }
  })
  rows = updateDiagnosticRows(rows, 'traceroute', {
    type: 'row',
    row: { hop: 2, host: '192.0.2.1', rtts_ms: [1, 3] }
  })
  assert.equal(rows.length, 2)
  assert.deepEqual(rows[0].rtts_ms, [1, 3])
  assert.equal(rows[1].host, '192.0.2.2')
})

test('Zero latency and timeout formatting', () => {
  assert.equal(formatMilliseconds(0), '0.00 ms')
  for (const value of [null, undefined, '-', NaN, Infinity])
    assert.equal(formatMilliseconds(value), '—')
  assert.equal(rowKey('ping', { seq: 0 }), 'ping:0')
  assert.notEqual(
    rowKey('nexttrace', { hop: 2, host: '*' }),
    rowKey('nexttrace', { hop: 2, host: '::1' })
  )
})
