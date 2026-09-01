import type { ReactNode } from 'react'

/**
 * The sanctioned vocabulary for node bodies. A node author composes these and
 * plain <p> / <ul> — never chrome, never a bespoke layout.
 */

/** A titled block inside an article. Renders the only h2 level nodes may use. */
export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="a-section">
      <h2>{title}</h2>
      {children}
    </section>
  )
}

/** Inline technology tags. */
export function Chips({ items }: { items: readonly string[] }) {
  if (items.length === 0) return null
  return (
    <ul className="a-chips">
      {items.map((chip) => (
        <li key={chip}>{chip}</li>
      ))}
    </ul>
  )
}

/** The one raised surface the design language allows. */
export function Figure({
  src,
  alt,
  caption,
  width,
  height,
  scroll = false,
}: {
  src: string
  alt: string
  caption?: string
  width?: number
  height?: number
  scroll?: boolean
}) {
  return (
    <figure className="a-figure">
      <div className={scroll ? 'a-figure-scroll' : undefined}>
        <img src={src} alt={alt} width={width} height={height} loading="lazy" decoding="async" />
      </div>
      {caption ? <figcaption>{caption}</figcaption> : null}
    </figure>
  )
}

/** An aside the reader should not skim past. */
export function Callout({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <aside className="a-callout">
      {title ? <p className="a-callout-title">{title}</p> : null}
      {children}
    </aside>
  )
}

/** A number, what it is, and how it was measured — the third line is the point. */
export function Metric({ value, label, how }: { value: string; label: string; how?: string }) {
  return (
    <li className="a-metric">
      <span className="a-metric-value">{value}</span>
      <span className="a-metric-label">{label}</span>
      {how ? <span className="a-metric-how">{how}</span> : null}
    </li>
  )
}

export function Metrics({ children }: { children: ReactNode }) {
  return <ul className="a-metrics">{children}</ul>
}

export function Timeline({ items }: { items: readonly { when: string; what: string; detail?: string }[] }) {
  return (
    <ol className="a-timeline">
      {items.map((item) => (
        <li key={`${item.when}-${item.what}`}>
          <span className="a-timeline-when">{item.when}</span>
          <span className="a-timeline-what">{item.what}</span>
          {item.detail ? <span className="a-timeline-detail">{item.detail}</span> : null}
        </li>
      ))}
    </ol>
  )
}

export function DefinitionList({ items }: { items: readonly { term: string; detail: ReactNode }[] }) {
  return (
    <dl className="a-deflist">
      {items.map((item) => (
        <div key={item.term}>
          <dt>{item.term}</dt>
          <dd>{item.detail}</dd>
        </div>
      ))}
    </dl>
  )
}
