import { useEffect } from 'react'

const ORIGIN = 'https://filenkov.store'

function setMeta(name: string, content: string) {
  let tag = document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!tag) {
    tag = document.createElement('meta')
    tag.setAttribute('name', name)
    document.head.appendChild(tag)
  }
  tag.setAttribute('content', content)
}

function setLink(rel: string, href: string) {
  let tag = document.head.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`)
  if (!tag) {
    tag = document.createElement('link')
    tag.setAttribute('rel', rel)
    document.head.appendChild(tag)
  }
  tag.setAttribute('href', href)
}

/** Per-node <title>, description and canonical. The registry knows them all at build time. */
export function useDocumentHead(head: { path: string; title: string; description: string }): void {
  const { path, title, description } = head
  useEffect(() => {
    document.title = title
    setMeta('description', description)
    setLink('canonical', `${ORIGIN}/${path}`)
  }, [path, title, description])
}
