import assert from 'node:assert/strict'
import test from 'node:test'
import { renderMarkdown } from './markdown.js'

test('renders sponsor text, links, heading anchors and highlighted code', () => {
  const html = renderMarkdown(
    '# Sponsor\n\n**Thanks** [website](https://example.com)\n\n```js\nconst answer = 42\n```'
  )
  assert.match(html, /<h1 id="sponsor"/)
  assert.match(html, /<strong>Thanks<\/strong>/)
  assert.match(html, /href="https:\/\/example.com"/)
  assert.match(html, /hljs-keyword/)
})

test('preserves extended Markdown supported by the previous renderer', () => {
  const html = renderMarkdown(
    '*[ALS]: Another Looking-glass Server\n\nALS :smile:\n\n' +
      '++inserted++ ==marked== H~2~O x^2^\n\n- [x] done\n\n' +
      'Term\n: Definition\n\nText[^note]\n\n[^note]: Footnote\n\n' +
      '[[toc]]\n\n# Section'
  )
  assert.match(html, /<abbr title="Another Looking-glass Server">ALS<\/abbr>/)
  assert.ok(html.includes('😄'))
  assert.match(html, /<ins>inserted<\/ins>/)
  assert.match(html, /<mark>marked<\/mark>/)
  assert.match(html, /H<sub>2<\/sub>O/)
  assert.match(html, /x<sup>2<\/sup>/)
  assert.match(html, /type="checkbox"/)
  assert.match(html, /<dd>Definition<\/dd>/)
  assert.match(html, /class="footnotes/)
  assert.match(html, /class="table-of-contents"/)
})

test('escapes raw HTML and rejects unsafe link protocols', () => {
  const html = renderMarkdown('<script>alert(1)</script>\n\n[click](javascript:alert(1))')
  assert.ok(!html.includes('<script>'))
  assert.ok(!html.includes('href="javascript:'))
  assert.ok(html.includes('&lt;script&gt;'))
})

test('updates rendered content without carrying headings across messages', () => {
  assert.equal(renderMarkdown(''), '')
  assert.match(renderMarkdown('# Sponsor'), /id="sponsor"/)
  assert.match(renderMarkdown('# Sponsor'), /id="sponsor"/)
  assert.match(renderMarkdown('Updated message'), /<p>Updated message<\/p>/)
})
