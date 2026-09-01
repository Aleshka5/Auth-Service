import type { NodeMeta } from '../../../types'
import { solutionBySlug } from './solutions'

const meta: NodeMeta = {
  title: 'Architecture',
  teaser: 'The live service map, box by box: TLS at the edge, the gateway, the product hosts, Redis and MinIO.',
  order: 20,
  seo: {
    title: 'Architecture · Alexey Filenkov',
    description:
      'The service map behind filenkov.store: Caddy terminating TLS, the Go auth gateway, product hosts, Redis and shared MinIO.',
  },
  /**
   * Ten near-identical solution pages would be ten folders for no benefit, so
   * `/about-project/architecture/<slug>` is served from this node's body.
   * Returning null for an unknown slug is what sends a bad URL back to the root.
   */
  dynamicChild: {
    resolve: (slug) => {
      const solution = solutionBySlug(slug)
      return solution ? { title: solution.title, teaser: solution.summary } : null
    },
  },
}

export default meta
