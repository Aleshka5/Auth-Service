import { Chips, Section } from '../../../../../components/article'

export default function InfraNode() {
  return (
    <>
      <p className="lede">
        One domain, TLS at the edge, object storage shared by the house. This layer exists so that adding a
        service is a config change rather than a project.
      </p>
      <p>
        Everything runs in Docker containers on private networks, with only the edge published. Nothing else
        binds a public port — not the auth API, not the SPA container, not the agent dashboard. That is a rule I
        keep rather than a diagram I drew once: the moment a second service is reachable directly, the identity
        story at the edge stops being true.
      </p>
      <Chips items={['Caddy', 'Docker', 'MinIO']} />

      <Section title="Edge & identity">
        <p>
          Caddy terminates HTTPS with Let’s Encrypt certificates and reverse-proxies everything to one gateway.
          The gateway does the routing that actually carries policy — which host, which path prefix, public or
          signed-in or admin — from a YAML file, so a new subdomain is a few lines of configuration and no new
          code. Adding the LLM workspace and the storage host to this setup took a config entry each.
        </p>
        <Chips items={['Let’s Encrypt', 'DNS']} />
      </Section>

      <Section title="Per-service roles">
        <p>
          Access is an enum per service, not one global flag: ADMIN, FAMILY, STRANGER, BLOCKED. The point of the
          split is that the blast radius of a decision stays small — I can block someone on file storage without
          locking them out of this resume, and a new sign-in starts as STRANGER rather than as anything I have to
          remember to revoke. A small admin UI raises or blocks a role on one host at a time.
        </p>
        <Chips items={['Role Manager']} />
      </Section>
    </>
  )
}
