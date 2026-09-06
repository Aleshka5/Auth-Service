/**
 * The boxes on the architecture map. A dozen near-identical pages would be a
 * dozen folders for no benefit, so they stay a data array and the node serves
 * `/about-project/architecture/:slug` from it (§9.4). Imported by `meta.ts`,
 * so this file must stay free of React and of asset imports.
 *
 * The order follows the map: edge first, then the per-host lanes, then the
 * shared data plane. A box that leaves the diagram leaves this array too — an
 * unknown slug resolves to null and the reader is sent back to the root.
 */
export type ArchitectureSolution = {
  slug: string
  title: string
  summary: string
  href?: string
}

export const architectureSolutions: ArchitectureSolution[] = [
  {
    slug: 'caddy-https',
    title: 'Caddy HTTPS',
    summary:
      'The only box on the external network. Terminates TLS with Let’s Encrypt on :80/:443 and hands every host to the gateway — it decides nothing.',
  },
  {
    slug: 'auth-gateway',
    title: 'Auth Gateway',
    summary:
      'Go, :8083. The single place identity is decided: YAML host/path routing, access-JWT check, incoming X-Auth-* stripped and its own injected.',
    href: '/auth',
  },
  {
    slug: 'auth-cache',
    title: 'Rejected cache',
    summary:
      'Redis on auth_network. Rejected tokens by jti and by user, OAuth CSRF state, IP rate-limit counters, and the auth_session → user map.',
  },
  {
    slug: 'hub',
    title: 'Hub (filenkov.store)',
    summary:
      'This page. A static Vue/Vite container on :80 plus the internal Go auth API on :8000 — OAuth, /api/me, refresh, logout. Neither port is published.',
  },
  {
    slug: 'storage',
    title: 'Storage',
    summary:
      'storage.filenkov.store: a Python app on :8000 behind its own SPA. Gets X-User-Id and X-Storage-Role from the gateway, blobs on the shared MinIO.',
    href: 'https://storage.filenkov.store',
  },
  {
    slug: 'mlflow',
    title: 'MLflow',
    summary:
      'Experiment tracking on :5000, reached with client-certificate validation rather than the browser cookie. Artifacts to MinIO, metadata to the shared Postgres.',
  },
  {
    slug: 'hermes',
    title: 'Hermes',
    summary:
      'Agent workspace on :8787. The gateway injects Authorization: Bearer with the same access JWT, so there is no second login. Talks to MLflow and LiteLLM.',
    href: '/hermes/sessions',
  },
  {
    slug: 'llm',
    title: 'LiteLLM',
    summary:
      'llm.filenkov.store → LiteLLM on :4000, fronting the local model runners on :4001 and :4002. Same cookie at the edge, own network, no token downstream.',
    href: 'https://llm.filenkov.store',
  },
  {
    slug: 'user-service',
    title: 'User-Service',
    summary:
      'The directory of people on user_network — UUID, google_sub, username, global and per-service roles — with its own Postgres and Redis. Never sees a JWT.',
  },
  {
    slug: 'common-data',
    title: 'Common Postgres & Redis',
    summary:
      'One Postgres and one Redis on common_network, shared by the product services. Each keeps its own schema; none of them keeps a session.',
  },
  {
    slug: 'minio',
    title: 'Common MinIO S3',
    summary: 'Shared object store on :9000 — home_storage buckets, MLflow artifacts, and service backups.',
  },
  {
    slug: 'observability',
    title: 'Prometheus & Grafana',
    summary:
      'observability_network. Prometheus scrapes the badged services — gateway, storage, Hermes, User-Service — and Grafana reads Prometheus. Nothing scrapes from outside.',
  },
]

export function solutionBySlug(slug: string | undefined): ArchitectureSolution | undefined {
  if (!slug) return undefined
  return architectureSolutions.find((item) => item.slug === slug)
}
