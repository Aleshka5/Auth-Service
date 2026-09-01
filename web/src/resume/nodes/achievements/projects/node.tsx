import { Section } from '../../../../components/article'

export default function ProjectsNode() {
  return (
    <>
      <p className="lede">
        Everything on this branch is running right now, on one domain, behind one Google sign-in. The
        page you are reading is one of the services.
      </p>
      <p>
        The shape is the same in every case. Caddy terminates TLS at the edge and reverse-proxies to the
        Auth Gateway. The gateway is the only public HTTP edge: it matches the host and path against a
        YAML routing table, checks the access JWT carried in an httpOnly cookie, applies IP rate limits,
        and then proxies to an internal upstream that is never published on a public interface. Before
        proxying it strips any incoming identity headers and injects its own, so a product service can
        trust <strong>X-Auth-User-Id</strong>, <strong>X-Auth-Email</strong>, <strong>X-Auth-Name</strong>{' '}
        and <strong>X-Auth-Role</strong> without doing any authentication of its own.
      </p>
      <Section title="Why the services do not each grow a login">
        <p>
          A new service on this domain writes no OAuth code and owns no password. It gets added to the
          routing table, it reads four headers, and it is done — adding a host is a config change, not a
          code change. That is the whole reason the hub exists, and it is what makes the smaller
          experiments on this branch cheap enough to be worth building at all.
        </p>
        <p>
          Authorisation stays with each service. The directory holds one global role for the hub and a
          separate role per service, so a person can be blocked on file storage and still read this
          resume. Signing in once does not mean being trusted everywhere; it means being{' '}
          <em>identified</em> everywhere, and each host decides the rest.
        </p>
      </Section>
      <p>
        Each project below states what it does, what it is built on, and where the interesting decision
        was. Where a service is publicly reachable, the article ends with a button to open it.
      </p>
    </>
  )
}
