import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  diagnosticURL,
  formatMilliseconds,
  latencyTone,
  lossTone,
  summarizePing,
  isValidTarget,
  normalizeTarget,
  rowKey,
  updateDiagnosticRows
} from './diagnostics.ts'

const targets = JSON.parse(
  readFileSync(new URL('./diagnostic-targets.json', import.meta.url), 'utf8')
) as Record<string, string[]>
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
  assert.deepEqual(rows[0]?.rtts_ms, [1, 3])
  assert.equal(rows[1]?.host, '192.0.2.2')
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

test('Latency and loss tones', () => {
  assert.equal(latencyTone(0), 'good')
  assert.equal(latencyTone(79.9), 'good')
  assert.equal(latencyTone(80), 'fair')
  assert.equal(latencyTone(200), 'poor')
  assert.equal(latencyTone(null), 'none')
  assert.equal(lossTone(0), 'good')
  assert.equal(lossTone(10), 'fair')
  assert.equal(lossTone(100), 'poor')
  assert.equal(lossTone(undefined), 'none')
})

test('Ping summary counts timeouts as loss', () => {
  assert.deepEqual(summarizePing([]), {
    sent: 0,
    received: 0,
    loss: 0,
    min: null,
    avg: null,
    max: null
  })
  const summary = summarizePing([
    { seq: 0, latency_ms: 10 },
    { seq: 1, latency_ms: 30 },
    { seq: 2, timeout: true },
    { seq: 3, latency_ms: 20 }
  ])
  assert.deepEqual(summary, { sent: 4, received: 3, loss: 25, min: 10, avg: 20, max: 30 })
})

test('Targets are trimmed and IDNs converted to Punycode before validation', () => {
  const cases: [string, string][] = [
    [' example.com', 'example.com'],
    ['example.com \t\r\n', 'example.com'],
    ['\u3000例子.测试\u3000', 'xn--fsqu00a.xn--0zwm56d'],
    ['例子.测试', 'xn--fsqu00a.xn--0zwm56d'],
    ['例子。测试', 'xn--fsqu00a.xn--0zwm56d'],
    ['Bücher.DE', 'xn--bcher-kva.de'],
    ['münchen.de.', 'xn--mnchen-3ya.de.'],
    ['ｅxample.com', 'example.com'],
    ['１２７.０.０.１', '127.0.0.1'],
    ['xn--fsqu00a.xn--0zwm56d', 'xn--fsqu00a.xn--0zwm56d'],
    ['2001:db8::1', '2001:db8::1']
  ]
  for (const [input, expected] of cases) {
    assert.equal(normalizeTarget(input), expected, JSON.stringify(input))
    assert.equal(isValidTarget(normalizeTarget(input)), true, JSON.stringify(input))
  }
})

test('Normalisation never turns URL-like or malformed input into a valid target', () => {
  for (const input of [
    '',
    '   ',
    '例子.测试:80',
    'http://例子.测试',
    '例子.测试/a',
    'user@例子.测试',
    '例子 .测试',
    '-例.com',
    '例-.com',
    '例.com;id',
    '--help',
    '1.1.1.1 && id',
    'example.com\n--output=/tmp/a'
  ]) {
    assert.equal(isValidTarget(normalizeTarget(input)), false, JSON.stringify(input))
  }
  // Pure ASCII is only trimmed, never rewritten.
  assert.equal(normalizeTarget(' EXAMPLE.COM '), 'EXAMPLE.COM')
})
