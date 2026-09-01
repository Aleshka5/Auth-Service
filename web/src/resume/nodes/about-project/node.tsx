import { Callout, Chips, Section } from '../../../components/article'

export default function AboutProjectNode() {
  return (
    <>
      <p className="lede">
        This page is not a description of my work. It is a piece of it — the same box on the same map as the file
        storage and the model workspace, served by the same edge.
      </p>

      <p>
        When you opened this URL, a Caddy container terminated TLS and handed the request to a Go binary I wrote
        (<code>cmd/gateway</code>). That gateway read a YAML file, matched the path against the route list for
        <strong> filenkov.store</strong>, decided the route was public, and proxied it to an nginx container holding the
        built SPA. Had you asked for <strong>/admin</strong> instead, the same code would have checked the access JWT in
        your cookie, found no <code>ADMIN</code> role, and sent you to the sign-in page before nginx ever saw the
        request. Nothing on this site is a mock-up of a system; the resume is a tenant of the system.
      </p>

      <Chips items={['React 19', 'Vite', 'TypeScript', 'Go', 'nginx', 'Caddy', 'Redis', 'Docker Compose']} />

      <Section title="Why it gets a branch">
        <p>
          Because it answers the question an interviewer actually has after reading the experience section: does he
          build things, or does he describe things? The honest way to answer it is to hand over something running, with
          its routing table, its auth decisions and its deploy in the same repository as the words about it.
        </p>
        <p>
          It also let me settle an argument with myself about resumes. A single scrolling page gives every reader the
          same words in the same order, which means it is optimised for nobody. This one is a tree you walk: you are two
          clicks from the design argument, the live service map, or the deployment details, and you will never scroll
          past the ones you did not ask for.
        </p>
      </Section>

      <Callout title="Everything here is checkable">
        <p>
          Versions, ports and routes quoted in these nodes are read out of <code>package.json</code>, <code>go.mod</code>
          {' '}and <code>deploy/gateway.yaml</code> in the repository, not from memory. If a number here is wrong, the
          repository is where it is wrong.
        </p>
      </Callout>
    </>
  )
}
