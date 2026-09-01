import type { NodeMeta } from '../../../../types'

const meta: NodeMeta = {
  title: 'LLM',
  meta: 'llm.filenkov.store',
  teaser: 'Personal LLM workspace behind the same auth gate.',
  order: 30,
  outbound: { label: 'Open LLM', href: 'https://llm.filenkov.store' },
  seo: {
    title: 'LLM · Projects',
    description: 'A personal model workspace on its own subdomain, with identity injected at the edge.',
  },
}

export default meta
