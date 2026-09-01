import { Link } from 'react-router-dom'
import { Figure } from '../../../../components/article'
import type { NodeProps } from '../../../types'
import architectureUrl from './architecture.svg?url'
import { architectureSolutions, solutionBySlug } from './solutions'

/**
 * One body, two modes. With a `slug` it renders a single box from the map
 * (§9.4 — no folder per solution); without one it renders the map itself.
 */
export default function ArchitectureNode({ slug }: NodeProps) {
  if (slug) return <SolutionBody slug={slug} />
  return <MapBody />
}

function SolutionBody({ slug }: { slug: string }) {
  const solution = solutionBySlug(slug)
  // resolve() in meta.ts should have kept an unknown slug from reaching here.
  if (!solution) return null

  const external = /^https?:\/\//.test(solution.href ?? '')

  return (
    <>
      <p className="lede">{solution.summary}</p>
      <p>
        A dedicated write-up for this box is not here yet, and I would rather say so than pad the page.
        The map stays the source of truth until it is written: where this box sits, what talks to it, and
        what it is trusted for are all readable from the diagram, and everything beyond that would be me
        describing intentions rather than a running service.
      </p>
      {solution.href ? (
        <p>
          <a href={solution.href} {...(external ? { rel: 'noreferrer' } : {})}>
            Open the service
          </a>{' '}
          — it is behind the same sign-in as this page, so what you see depends on the role attached to
          your account.
        </p>
      ) : null}
    </>
  )
}

function MapBody() {
  return (
    <>
      <p className="lede">
        The house as it runs today, not as it is planned. Every box below is a container that exists, on a
        network that exists, reachable at an address you can type.
      </p>

      <Figure
        src={architectureUrl}
        alt="Filenkov service map: Caddy TLS in front of the Auth Gateway, product hosts, databases, Redis cache, and MinIO."
        caption="The live map. Scroll it sideways if the screen is narrow."
        scroll
      />

      <p>
        Read it left to right. DNS points the hub and its subdomains at one machine; Caddy terminates TLS
        there and does nothing clever afterwards — it hands every request to the Go auth gateway. The
        gateway is the only component that decides anything about identity: it matches the host and path
        against a YAML route list, checks the access JWT when the route is not public, strips whatever{' '}
        <code>X-Auth-*</code> headers the client tried to send, and injects its own. Everything to the right
        of it — the storage host, the model workspace, this resume — is a plain service that trusts those
        headers and never sees a token.
      </p>

      <p>
        The consequence worth noticing is that adding a product is a config change. A new host gets a block
        in the routing file and a container on the network; it does not get its own login, its own session
        store, or its own opinion about roles.
      </p>

      <ul>
        {architectureSolutions.map((item) => (
          <li key={item.slug}>
            <Link to={`/about-project/architecture/${item.slug}`}>
              <strong>{item.title}</strong>
            </Link>{' '}
            — {item.summary}
          </li>
        ))}
      </ul>
    </>
  )
}
