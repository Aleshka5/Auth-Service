import type { NodeMeta } from '../../../../types'

const meta: NodeMeta = {
  title: 'Hermes',
  meta: 'hermes.filenkov.store',
  teaser: 'Agent dashboard behind the same Google sign-in. Listed only to signed-in readers.',
  order: 50,
  visibleWhen: 'signed-in',
  outbound: { label: 'Open Hermes', href: 'https://hermes.filenkov.store' },
  seo: {
    title: 'Hermes · Projects',
    description:
      'An agent dashboard on its own host under the same domain, sharing the same Google sign-in, with a terminal session carried over a WebSocket.',
  },
}

export default meta
