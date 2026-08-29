import { useId, useState } from 'react'
import type { TreeItem } from '../resume/data'

type RevealTreeProps = {
  items: TreeItem[]
  labelledBy?: string
}

export function RevealTree({ items, labelledBy }: RevealTreeProps) {
  return (
    <ul className="reveal-tree" aria-labelledby={labelledBy}>
      {items.map((item) => (
        <RevealNode key={item.id} item={item} depth={0} />
      ))}
    </ul>
  )
}

function RevealNode({ item, depth }: { item: TreeItem; depth: number }) {
  const reactId = useId()
  const panelId = `${reactId}-children`
  const hasChildren = Boolean(item.children?.length)
  const [open, setOpen] = useState(false)

  return (
    <li className="reveal-node" data-depth={depth}>
      <div className="reveal-row">
        {hasChildren ? (
          <button
            type="button"
            className="reveal-toggle"
            aria-expanded={open}
            aria-controls={panelId}
            onClick={() => setOpen((value) => !value)}
          >
            <span className="reveal-chevron" aria-hidden="true">
              {open ? '▾' : '▸'}
            </span>
            <span className="reveal-heading">
              <span className="reveal-title">{item.title}</span>
              {item.meta ? <span className="reveal-meta">{item.meta}</span> : null}
            </span>
          </button>
        ) : (
          <div className="reveal-heading reveal-heading-static">
            <span className="reveal-title">{item.title}</span>
            {item.meta ? <span className="reveal-meta">{item.meta}</span> : null}
          </div>
        )}
        {item.href ? (
          <a className="reveal-link" href={item.href}>
            Open {item.title}
          </a>
        ) : null}
      </div>
      {item.summary ? <p className="reveal-summary">{item.summary}</p> : null}
      {item.chips?.length ? (
        <ul className="reveal-chips">
          {item.chips.map((chip) => (
            <li key={chip}>{chip}</li>
          ))}
        </ul>
      ) : null}
      {hasChildren && open ? (
        <div id={panelId} className="reveal-children">
          <p className="reveal-hint">Deeper — more technical</p>
          <ul className="reveal-tree">
            {item.children!.map((child) => (
              <RevealNode key={child.id} item={child} depth={depth + 1} />
            ))}
          </ul>
        </div>
      ) : null}
    </li>
  )
}
