/**
 * The boxes on the architecture map. Ten near-identical pages would be ten
 * folders for no benefit, so they stay a data array and the node serves
 * `/about-project/architecture/:slug` from it (§9.4). Imported by `meta.ts`,
 * so this file must stay free of React and of asset imports.
 */
export type ArchitectureSolution = {
  slug: string
  title: string
  summary: string
  href?: string
}

export const architectureSolutions: ArchitectureSolution[] = [
  {
    slug: 'auth-gateway',
    title: 'Auth Gateway',
    summary: 'Google OAuth2 hub at filenkov.store/auth. Caddy terminates TLS; this gateway routes and injects identity.',
    href: '/auth',
  },
  {
    slug: 'caddy-https',
    title: 'Caddy HTTPS',
    summary: 'TLS at the edge with Let’s Encrypt. Reverse-proxies every host to this service’s gateway.',
  },
  {
    slug: 'auth-db',
    title: 'User-Service',
    summary: 'Directory of people (UUID, google_sub, username, roles). Auth has no Postgres. Gateway X-Auth-Role is the access-JWT global_role.',
  },
  {
    slug: 'auth-cache',
    title: 'Auth Cache',
    summary: 'Redis rejected tokens, OAuth state, and IP rate limits.',
  },
  {
    slug: 'role-manager',
    title: 'Role Manager',
    summary: 'Admin UI to raise or BLOCK a role on one host without touching the others.',
  },
  {
    slug: 'storage',
    title: 'Storage',
    summary: 'Home file cloud. Sits behind the gateway and trusts X-Auth-* headers; blobs on shared MinIO.',
    href: 'https://storage.filenkov.store',
  },
  {
    slug: 'llm',
    title: 'LLM',
    summary: 'Personal model workspace. Same cookie, own database, identity injected at the edge.',
    href: 'https://llm.filenkov.store',
  },
  {
    slug: 'resume',
    title: 'Resume Page',
    summary: 'Vacancy-aware resume and cover-letter studio. Will grow its own architecture page.',
  },
  {
    slug: 'minio',
    title: 'Common MinIO S3',
    summary: 'Shared object store: home_storage, resume artifacts, and service backups.',
  },
  {
    slug: 'dns',
    title: 'DNS',
    summary: 'filenkov.store and the product subdomains. The map starts here.',
  },
]

export function solutionBySlug(slug: string | undefined): ArchitectureSolution | undefined {
  if (!slug) return undefined
  return architectureSolutions.find((item) => item.slug === slug)
}
