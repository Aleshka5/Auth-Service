import type { NodeMeta } from '../../../../types'

const meta: NodeMeta = {
  title: 'Hermes',
  meta: 'filenkov.store/hermes/chat',
  teaser: 'Agent dashboard behind the same Google sign-in. Listed only to signed-in readers.',
  order: 50,
  visibleWhen: 'signed-in',
  outbound: { label: 'Open Hermes', href: '/hermes/chat' },
  seo: {
    title: 'Hermes · Projects',
    description:
      'An agent dashboard on a path under the hub, sharing the same Google sign-in, with a terminal session carried over a WebSocket through the gateway.',
  },
}

export default meta
