import { Chips, Section } from '../../../../../components/article'

export default function AuthHubNode() {
  return (
    <>
      <p className="lede">
        One Google sign-in for every host I run. The Auth Gateway sits in front of them and injects who
        you are — there are no tokens in the URL and no second login anywhere.
      </p>
      <p>
        Caddy terminates HTTPS with Let&rsquo;s Encrypt and reverse-proxies everything to the gateway,
        which is the only publicly reachable HTTP service. The OAuth API stays internal and unpublished;
        so does the static SPA container. Routing lives in YAML, first matching route wins, and adding a
        host is a config change rather than a code change.
      </p>

      <Section title="JWT cookies">
        <p>
          Edge credentials are two httpOnly cookies, <strong>auth_access</strong> and{' '}
          <strong>auth_refresh</strong>, scoped to <strong>.filenkov.store</strong> with SameSite=Lax and
          Secure in production. Nothing lives in localStorage and nothing is sent in an Authorization
          header from the browser. On every proxied request the gateway strips incoming X-Auth-* headers
          and injects its own, so an upstream cannot be spoofed by a client that invents them.
        </p>
        <p>
          Redis carries the parts that must be revocable or rate-limited rather than merely signed: the
          rejected-token cache keyed by JWT id and by user UUID, the OAuth CSRF state, and the per-IP
          counters. A signed-out or blocked user stops being proxied on the next request instead of
          waiting for a token to expire.
        </p>
        <Chips items={['Go', 'Redis']} />
      </Section>

      <Section title="Hub and service roles">
        <p>
          User-Service is the source of truth for people — UUID, google_sub, email, username, a global
          role, and a separate role per service. The auth side keeps no user database and no role columns
          of its own, and User-Service never receives a JWT. The identity the gateway puts at the edge is
          the access token&rsquo;s global_role.
        </p>
        <p>
          The first person in the directory becomes ADMIN; every later Google login starts as STRANGER.
          Because service roles are stored separately, BLOCKED on one service does not lock the hub —
          somebody can lose file storage and still open this page.
        </p>
        <Chips items={['ADMIN / FAMILY / STRANGER / BLOCKED', 'Rate limit by IP']} />
      </Section>
    </>
  )
}
