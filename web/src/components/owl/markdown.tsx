import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { normalizePath, resolve } from '../../resume/registry'
import type { User } from '../../types'
import { withNodeLinks } from './nodeRefs'

/**
 * A small markdown renderer for the owl's answers.
 *
 * The model writes markdown, so the panel has to read it. This is deliberately
 * hand-written rather than a library: the answer arrives a token at a time and
 * is re-rendered on every delta, the subset a chat answer uses is small, and
 * nothing here can ever emit raw HTML — the output is React elements only, so
 * an answer that contains a `<script>` is text and stays text.
 *
 * Plain text runs still go through `withNodeLinks`, so page paths the model
 * mentions in prose keep linking into the tree.
 */

/** Emphasis, code spans and explicit links, in one pass. */
const INLINE =
  /(`+)([\s\S]*?)\1|\[([^\]]*)\]\(([^()\s]+)\)|(\*\*|__)(?=\S)([\s\S]*?\S)\5|(\*|_)(?=\S)([\s\S]*?\S)\7|~~(?=\S)([\s\S]*?\S)~~/g

/** A link the model wrote, as opposed to a bare path in prose. */
function renderLink(label: string, href: string, user: User | null, key: string): ReactNode {
  if (href.startsWith('/')) {
    const path = normalizePath(href)
    const node = path === '' ? null : resolve(path, user)
    // An invented page, or one this reader may not see, is not a link.
    if (!node) return label
    return (
      <Link key={key} className="owl-chat-ref" to={`/${path}`} title={node.title}>
        {label}
      </Link>
    )
  }
  // Only the two schemes a browser can safely follow; `javascript:` and friends
  // fall through to plain text.
  if (!/^https?:\/\//i.test(href) && !href.startsWith('mailto:')) return label
  return (
    <a key={key} className="owl-chat-ref" href={href} target="_blank" rel="noreferrer noopener">
      {label}
    </a>
  )
}

function inline(text: string, user: User | null, keyBase: string): ReactNode[] {
  const out: ReactNode[] = []
  let cursor = 0

  for (const m of text.matchAll(INLINE)) {
    const start = m.index
    if (start > cursor) out.push(...withNodeLinks(text.slice(cursor, start), user))
    const key = `${keyBase}-${start}`

    if (m[1] !== undefined) {
      out.push(
        <code key={key} className="owl-chat-code">
          {m[2].trim()}
        </code>,
      )
    } else if (m[4] !== undefined) {
      out.push(renderLink(m[3], m[4], user, key))
    } else if (m[5] !== undefined) {
      out.push(<strong key={key}>{inline(m[6], user, key)}</strong>)
    } else if (m[7] !== undefined) {
      out.push(<em key={key}>{inline(m[8], user, key)}</em>)
    } else {
      out.push(<del key={key}>{inline(m[9], user, key)}</del>)
    }
    cursor = start + m[0].length
  }

  if (cursor < text.length) out.push(...withNodeLinks(text.slice(cursor), user))
  return out
}

const HEADING = /^(#{1,6})\s+(.*)$/
const BULLET = /^[-*+]\s+(.*)$/
const ORDERED = /^(\d+)[.)]\s+(.*)$/
const RULE = /^(?:-{3,}|\*{3,}|_{3,})$/
const FENCE = /^\s*(`{3,}|~{3,})\s*([\w+-]*)\s*$/
const TABLE_DIVIDER = /^\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?$/

/** Splits a table row into its cells, tolerating the optional outer pipes. */
function cells(line: string): string[] {
  return line
    .replace(/^\s*\|/, '')
    .replace(/\|\s*$/, '')
    .split('|')
    .map((c) => c.trim())
}

/** How deep a list line is indented, in whole levels of two spaces. */
function indentOf(line: string): number {
  const spaces = line.match(/^[ \t]*/)?.[0] ?? ''
  return Math.floor(spaces.replace(/\t/g, '  ').length / 2)
}

function blocks(lines: string[], user: User | null, keyBase: string): ReactNode[] {
  const out: ReactNode[] = []
  let i = 0

  const key = () => `${keyBase}-${i}`

  while (i < lines.length) {
    const line = lines[i]
    const trimmed = line.trim()

    if (trimmed === '') {
      i += 1
      continue
    }

    const fence = FENCE.exec(line)
    if (fence) {
      const marker = fence[1]
      const body: string[] = []
      i += 1
      // An unclosed fence is normal mid-stream: take the rest and keep going.
      while (i < lines.length && !lines[i].trim().startsWith(marker)) {
        body.push(lines[i])
        i += 1
      }
      i += 1
      out.push(
        <pre key={key()} className="owl-chat-pre">
          <code>{body.join('\n')}</code>
        </pre>,
      )
      continue
    }

    if (RULE.test(trimmed)) {
      out.push(<hr key={key()} className="owl-chat-rule" />)
      i += 1
      continue
    }

    const heading = HEADING.exec(trimmed)
    if (heading) {
      // The panel is a small box inside a page, so the answer starts at h3 and
      // never competes with the document's own headings.
      const level = Math.min(6, heading[1].length + 2)
      const Tag = `h${level}` as 'h3'
      out.push(
        <Tag key={key()} className="owl-chat-heading">
          {inline(heading[2], user, key())}
        </Tag>,
      )
      i += 1
      continue
    }

    if (trimmed.startsWith('>')) {
      const quoted: string[] = []
      while (i < lines.length && lines[i].trim().startsWith('>')) {
        quoted.push(lines[i].trim().replace(/^>\s?/, ''))
        i += 1
      }
      out.push(
        <blockquote key={key()} className="owl-chat-quote">
          {blocks(quoted, user, `${key()}-q`)}
        </blockquote>,
      )
      continue
    }

    // A table needs its divider row to be a table; without one these are just
    // paragraph lines that happen to contain pipes.
    if (trimmed.includes('|') && i + 1 < lines.length && TABLE_DIVIDER.test(lines[i + 1].trim())) {
      const header = cells(trimmed)
      const rows: string[][] = []
      i += 2
      while (i < lines.length && lines[i].includes('|') && lines[i].trim() !== '') {
        rows.push(cells(lines[i].trim()))
        i += 1
      }
      const k = key()
      out.push(
        <div key={k} className="owl-chat-table-wrap">
          <table className="owl-chat-table">
            <thead>
              <tr>
                {header.map((c, n) => (
                  <th key={n} scope="col">
                    {inline(c, user, `${k}-h${n}`)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, r) => (
                <tr key={r}>
                  {row.map((c, n) => (
                    <td key={n}>{inline(c, user, `${k}-${r}-${n}`)}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>,
      )
      continue
    }

    const bullet = BULLET.exec(trimmed)
    const ordered = ORDERED.exec(trimmed)
    if (bullet || ordered) {
      const depth = indentOf(line)
      const items: ReactNode[] = []
      const k = key()
      let n = 0

      // One list runs until a line that is neither an item at this depth nor a
      // continuation of the current one.
      while (i < lines.length) {
        const raw = lines[i]
        const t = raw.trim()
        if (t === '') break
        const b = BULLET.exec(t)
        const o = ORDERED.exec(t)
        if (!b && !o) break
        if (indentOf(raw) !== depth) break
        if (Boolean(b) !== Boolean(bullet)) break

        const own: string[] = [(b ?? o!)[b ? 1 : 2]]
        i += 1
        // Deeper items and wrapped lines belong to the item just opened.
        while (i < lines.length && lines[i].trim() !== '' && indentOf(lines[i]) > depth) {
          own.push(lines[i].slice(2))
          i += 1
        }
        items.push(
          <li key={`${k}-${n}`}>
            {own.length === 1
              ? inline(own[0], user, `${k}-${n}`)
              : blocks([own[0], ...own.slice(1)], user, `${k}-${n}`)}
          </li>,
        )
        n += 1
      }

      out.push(
        ordered ? (
          <ol key={k} className="owl-chat-list" start={Number(ordered[1])}>
            {items}
          </ol>
        ) : (
          <ul key={k} className="owl-chat-list">
            {items}
          </ul>
        ),
      )
      continue
    }

    // Anything else is a paragraph, running to the next blank line or block.
    const para: string[] = []
    while (i < lines.length) {
      const t = lines[i].trim()
      if (t === '' || HEADING.test(t) || BULLET.test(t) || ORDERED.test(t) || RULE.test(t) || FENCE.test(lines[i]) || t.startsWith('>')) {
        break
      }
      para.push(t)
      i += 1
    }
    out.push(
      <p key={key()} className="owl-chat-para">
        {inline(para.join('\n'), user, key())}
      </p>,
    )
  }

  return out
}

/** Renders one markdown answer, streamed or complete. */
export function renderMarkdown(text: string, user: User | null): ReactNode[] {
  return blocks(text.replace(/\r\n?/g, '\n').split('\n'), user, 'md')
}
