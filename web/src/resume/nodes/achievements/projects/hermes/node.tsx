import { Callout } from '../../../../../components/article'

export default function HermesNode() {
  return (
    <>
      <p className="lede">
        An agent dashboard on its own host, at <strong>hermes.filenkov.store</strong>. You can see this
        page because you are signed in; a guest is not shown it at all.
      </p>
      <p>
        Unlike the other projects on this branch, Hermes is not something I wrote — it is a dashboard I
        run, and the work here was putting it behind the same door as everything else. It arrived with its
        own idea of authentication, and the requirement was that it must not add a second login: one
        Google sign-in at the hub, and every service on the domain accepts it.
      </p>
      <p>
        The gateway used to do that by routing <strong>/hermes/*</strong> as a session route, injecting the
        access JWT as a Bearer token and telling the app it was mounted under a prefix, so its own auth
        plugin verified the hub’s token rather than issuing one. That mount was retired in October 2026 in
        favour of this dedicated host; the dashboard keeps the same plugin, and it is still never published
        on a public interface.
      </p>
      <p>
        The chat page is the interesting exception. It is a terminal, so it is not ordinary HTTP: the
        browser mints a single-use ticket over HTTP and then opens a WebSocket through the same gateway.
        Getting that upgrade to survive the proxy is where most of the real work went — an upgrade has to
        stay on HTTP/1.1, response rewriting and security headers have to be skipped on the 101, and an
        anonymous attempt has to fail as a 401 rather than as a redirect a socket cannot follow.
      </p>
      <Callout title="Why this node is signed-in only">
        <p>
          Hermes is not a public service, and a link a visitor cannot open is noise. The registry filters
          it out for guests declaratively, which is the same visibility rule the gateway enforces at the
          edge — stated twice, in the two places that each have to know.
        </p>
      </Callout>
    </>
  )
}
