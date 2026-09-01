import type { NodeMeta } from '../../../../types'

const meta: NodeMeta = {
  title: 'Reports that remember',
  teaser: 'Memory links reports to each other; RAG covers the domain so later runs stay consistent.',
  order: 20,
  seo: {
    title: 'Reports that remember · Sber',
    description:
      'Memory between report runs plus Faiss-backed RAG over the domain corpus, so a later report agrees with the one before it.',
  },
}

export default meta
