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
        alt="Service map: Caddy terminating TLS on the external network, then the Go auth gateway routing filenkov.store, storage.filenkov.store and llm.filenkov.store to their containers, with User-Service, a shared Postgres, Redis and MinIO, and a Prometheus/Grafana network below."
        caption="The shape of it at a glance — open the full map for the labels."
      />

      <p>
        The diagram is drawn far wider than a column of prose, so it is here at the size of the column and{' '}
        <a href={architectureUrl} target="_blank" rel="noreferrer">
          full size in its own tab
        </a>
        . The paragraphs below say the same thing in words, and they are the version worth reading if you
        are on a phone.
      </p>

      <p>
        There is one line across the top and everything interesting is under it. On the external network
        Caddy holds the certificates and terminates TLS on <code>:80</code> and <code>:443</code>; it does
        nothing clever afterwards. Every host it answers for is handed to the same Go gateway on{' '}
        <code>:8083</code>, which is the only component that decides anything about identity: it matches
        host and path against a YAML route list, checks the access JWT when the route is not public, looks
        the token up in the rejected cache next to it, strips whatever <code>X-Auth-*</code> headers the
        client tried to send, and injects its own.
      </p>

      <p>
        Below the gateway the map fans out into one lane per host, and each lane is its own compose
        network. <code>filenkov.store</code> is this SPA plus the internal auth API that talks to Google.{' '}
        <code>storage.filenkov.store</code> is the file cloud, which additionally receives the role it is
        allowed to act on rather than the hub role. <code>llm.filenkov.store</code> is a llama.cpp
        server that only a hub admin may open. LiteLLM stays on the internal network for the owl
        chat. Hermes gets the access JWT as a bearer header so the agent workspace does
        not ask for a second login, and MLflow sits behind client-certificate validation instead of the
        cookie. None of these services sees a token it has to verify itself, and none of them holds a
        session.
      </p>

      <p>
        The bottom row is the part that is deliberately boring. User-Service owns the people — UUID,{' '}
        <code>google_sub</code>, username, roles — on its own network with its own Postgres and Redis, and
        it is the one component that is never sent a JWT. Next to it a shared Postgres, Redis and MinIO
        back the products themselves, and a separate observability network scrapes whatever exposes
        metrics. The consequence worth noticing is that adding a product is a config change: a new host
        gets a block in the routing file and a container on the network. It does not get its own login,
        its own session store, or its own opinion about roles.
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
