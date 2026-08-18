import { useAuth } from '../auth'
import { isAdmin } from '../types'

const publicLinks = [
  { n: '01', name: 'LLM', href: 'https://llm.filenkov.store', host: 'llm.filenkov.store' },
  { n: '02', name: 'Storage', href: 'https://storage.filenkov.store', host: 'storage.filenkov.store' },
] as const

export function Home() {
  const { user } = useAuth()
  const showHermes = isAdmin(user)

  return (
    <>
      <section className="hero">
        <p className="kicker">Studio hub</p>
        <h1>Tools on one domain, one sign-in.</h1>
        <p className="lede">
          Filenkov is a small set of personal services. This page is public; Google unlocks the rest
          of the house.
        </p>
      </section>

      <div className="grid-2">
        <section className="about" aria-labelledby="about-heading">
          <h2 id="about-heading">About</h2>
          <p>
            Sign in once with Google. Other Filenkov sites ask this hub who you are — no tokens in
            the URL, no extra passwords.
          </p>
          <p>
            New accounts start as strangers. An admin can raise roles for storage, resume, and this
            site itself. Hermes stays off the public list.
          </p>
        </section>

        <section aria-labelledby="links-heading">
          <h2 id="links-heading">Elsewhere</h2>
          <ul className="destinations">
            {publicLinks.map((link) => (
              <li key={link.href}>
                <a href={link.href}>
                  <span className="dest-num">{link.n}</span>
                  <span className="dest-name">{link.name}</span>
                  <span className="dest-host">{link.host}</span>
                </a>
              </li>
            ))}
            {showHermes ? (
              <li>
                <a href="/hermes/">
                  <span className="dest-num">03</span>
                  <span className="dest-name">Hermes</span>
                  <span className="dest-host">filenkov.store/hermes/</span>
                </a>
              </li>
            ) : null}
          </ul>
        </section>
      </div>
    </>
  )
}
