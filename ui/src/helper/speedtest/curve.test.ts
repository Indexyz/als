import assert from 'node:assert/strict'
import { test } from 'node:test'
import { curvePath, trendGeometry } from './curve.ts'

test('curve path starts at the first point and ends at the last', () => {
  assert.equal(curvePath([]), '')
  const d = curvePath([
    { x: 0, y: 10 },
    { x: 5, y: 0 },
    { x: 10, y: 10 }
  ])
  assert.match(d, /^M0,10 C/)
  assert.match(d, / 10,10$/)
  assert.equal((d.match(/C/g) ?? []).length, 2)
})

test('trend geometry normalises to the peak and places the marker', () => {
  const empty = trendGeometry([], 100, null, 100, 40)
  assert.deepEqual(empty, { line: '', area: '', markerY: null })

  const geometry = trendGeometry([0, 50, 100], 3, 50, 100, 40, 1)
  assert.match(geometry.line, /^M0,40 C/)
  // Peak (100) sits at 10% headroom from the top.
  assert.match(geometry.line, / 100,4$/)
  assert.equal(geometry.markerY, 22)
  assert.match(geometry.area, /L100,40 L0,40 Z$/)
})

test('trend geometry always includes the last sample when striding', () => {
  const geometry = trendGeometry([1, 2, 3, 4], 10, null, 90, 10, 2)
  // Points at i=0, 2 and the trailing i=3.
  assert.equal((geometry.line.match(/C/g) ?? []).length, 2)
  assert.match(geometry.line, / 30,1$/)
})
