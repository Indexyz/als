import MarkdownIt from 'markdown-it'
import abbr from 'markdown-it-abbr'
import anchor from 'markdown-it-anchor'
import deflist from 'markdown-it-deflist'
import { full as emoji } from 'markdown-it-emoji'
import footnote from 'markdown-it-footnote'
import highlight from 'markdown-it-highlightjs'
import ins from 'markdown-it-ins'
import mark from 'markdown-it-mark'
import sub from 'markdown-it-sub'
import sup from 'markdown-it-sup'
import tasklists from 'markdown-it-task-lists'
import toc from 'markdown-it-toc-done-right'

// Preserve the sponsor message features previously provided by vue3-markdown-it.
// Raw HTML stays disabled; markdown-it also rejects unsafe link protocols.
const markdown = new MarkdownIt({ html: false })
  .use(abbr)
  .use(anchor)
  .use(deflist)
  .use(emoji)
  .use(footnote)
  .use(highlight)
  .use(ins)
  .use(mark)
  .use(sub)
  .use(sup)
  .use(tasklists)
  .use(toc)

export function renderMarkdown(source) {
  return markdown.render(source)
}
