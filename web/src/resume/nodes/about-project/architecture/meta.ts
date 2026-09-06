import type { NodeMeta } from '../../../types'
import { solutionBySlug } from './solutions'

const meta: NodeMeta = {
  title: 'Architecture',
  teaser: 'The live service map, box by box: TLS at the edge, the gateway, the product hosts, and the shared data plane under them.',
  order: 20,
  seo: {
    title: 'Architecture · Alexey Filenkov',
    description:
      'The service map behind filenkov.store: Caddy terminating TLS, the Go auth gateway, the storage, LLM, Hermes and MLflow lanes, User-Service, and the shared Postgres, Redis and MinIO.',
  },
  /**
   * A dozen near-identical solution pages would be a dozen folders for no benefit, so
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
