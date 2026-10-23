import assert from 'node:assert/strict'
import { test } from 'node:test'
import { formatBytes, splitUnit } from './unit.ts'

test('formats byte sizes with base 1024', () => {
  assert.equal(formatBytes(0), '0 Bytes')
  assert.equal(formatBytes(512), '512 Bytes')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes('1048576'), '1 MB')
})

test('formats bandwidth as bits per second with base 1000', () => {
  assert.equal(formatBytes(0, 2, true), '0 bps')
  assert.equal(formatBytes(125, 2, true), '1 Kbps')
  assert.equal(formatBytes(125_000, 2, true), '1 Mbps')
  assert.equal(formatBytes(1_250_000_000, 2, true), '10 Gbps')
  assert.equal(formatBytes(0.5, 2, true), '4 bps')
})

test('invalid or negative input never yields NaN or undefined units', () => {
  for (const value of [-1, NaN, Infinity, 'abc']) {
    assert.equal(formatBytes(value), '0 Bytes')
    assert.equal(formatBytes(value, 2, true), '0 bps')
  }
  assert.equal(formatBytes(0.25), '0.25 Bytes')
})

test('splitUnit separates number and unit', () => {
  assert.deepEqual(splitUnit('12.5 Mbps'), { value: '12.5', unit: 'Mbps' })
})
