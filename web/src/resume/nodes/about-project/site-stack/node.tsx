import { Callout, Chips, DefinitionList, Section } from '../../../../components/article'

export default function SiteStackNode() {
  return (
    <>
      <p className="lede">
        Two binaries, two containers of somebody else’s software, and a directory service. The front end is
        static files; every decision that matters happens before those files are served.
      </p>

      <Chips
        items={[
          'React 19',
          'react-router 7',
          'Vite 8',
          'TypeScript 6',
          'Go 1.25',
          'nginx 1.27',
          'Caddy',
          'Redis 7',
          'Docker Compose',
        ]}
      />

      <p>
        The site you are reading is a React 19 application built by Vite and routed by react-router. It is
        compiled once — <code>tsc -b &amp;&amp; vite build</code> inside a <code>node:22-alpine</code> stage —
        and the resulting <code>dist</code> is copied into an <code>nginx:1.27-alpine</code> image that
        listens on port 80 and is never published to the internet. nginx here is a file server and nothing
        else: no PHP, no proxy pass, no auth module.
      </p>

      <Section title="Who decides what">
        <DefinitionList
          items={[
            {
              term: 'Caddy',
              detail:
                'Terminates TLS with Let’s Encrypt and redirects HTTP to HTTPS. It does not choose between the SPA, the OAuth API and the other hosts — it hands everything to the gateway.',
            },
            {
              term: 'Gateway (cmd/gateway)',
              detail:
                'The only public HTTP edge, on :8083. Matches host and path against deploy/gateway.yaml, verifies the access JWT for non-public routes, strips inbound X-Auth-* headers and injects X-Auth-User-Id, X-Auth-Email, X-Auth-Name and X-Auth-Role, enforces an IP rate limit of 180 requests a minute, and writes JSON access logs.',
            },
            {
              term: 'API (cmd/api)',
              detail:
                'Internal only, on :8000, unpublished. Google OAuth2, /api/me, /api/refresh, /api/logout, /health. It issues the two httpOnly cookies, auth_access and auth_refresh, and shares the signing secret with the gateway alone.',
            },
            {
              term: 'User-Service',
              detail:
                'The directory of people — UUID, google_sub, email, username, roles. It is the source of truth for who exists, and it never receives a JWT.',
            },
            {
              term: 'Redis 7',
              detail:
                'Rejected tokens (by jti and by user), OAuth CSRF state, and the IP rate-limit counters. It is deliberately not the edge session store.',
            },
          ]}
        />
      </Section>

      <Section title="Why every URL in this tree just works">
        <p>
          A tree of nodes means a tree of real URLs, and a reader may land on any of them cold. That would
          normally be a routing change in two places. It was a change in neither, because{' '}
          <code>web/nginx.conf</code> already ends its location block with{' '}
          <code>try_files $uri $uri/ /index.html</code> — an unknown path falls back to the app shell and
          react-router takes it from there — and <code>deploy/gateway.yaml</code> already ends the hub’s route
          list with <code>match: /*</code>, <code>upstream: web</code>, <code>auth: public</code>. Every node
          added since is served by rules that were written before the tree existed.
        </p>
      </Section>

      <Callout title="What the Go side is actually made of">
        <p>
          Both binaries live in one module on Go 1.25, and the dependency list is short on purpose:{' '}
          <code>golang-jwt/jwt/v5</code> for the tokens, <code>redis/go-redis/v9</code> for the cache,{' '}
          <code>golang.org/x/oauth2</code> for the Google flow, <code>gopkg.in/yaml.v3</code> for the routing
          file, and <code>google/uuid</code>. No web framework, no ORM. The gateway is a router and a policy,
          and it is easier to keep it honest when it has nothing else in it.
        </p>
      </Callout>
    </>
  )
}
