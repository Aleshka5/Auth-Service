import { Section } from '../../../../../components/article'

export default function LlmNode() {
  return (
    <>
      <p className="lede">
        A personal model workspace on its own subdomain, sitting behind the same auth gate as everything
        else on the domain.
      </p>
      <p>
        The host is a proxy route like any other. The gateway matches{' '}
        <strong>llm.filenkov.store</strong>, requires a signed-in user, and forwards to the model proxy
        on an internal network — the proxy port is never published on a public interface. The model stack
        keeps its own configuration and its own idea of which backends exist; the only thing it receives
        from the edge is a set of identity headers describing who is asking.
      </p>
      <p>
        Keeping the workspace on a separate subdomain rather than a path under the hub was a deliberate
        choice. It means the model stack can be restarted, replaced, or pointed at a different provider
        without touching the routing of the resume or the storage service, and a failure there is visibly
        a failure of that one host.
      </p>

      <Section title="Auth at the edge">
        <p>
          Identity is decided once, at the edge, and never re-derived downstream. The gateway validates
          the access cookie, strips whatever identity headers the client sent, and injects the hub role
          it computed itself. The workspace trusts those headers because nothing else can reach it.
        </p>
        <p>
          The consequence is that the model stack stays off the public role list until it actually needs
          one. Services that require finer-grained permissions — storage is the current example — get a
          dedicated per-service role in the directory and a gateway rule that fetches it. This one does
          not, so it does not carry one, and the day it needs FAMILY-versus-STRANGER behaviour that is a
          directory key and a routing line rather than a rewrite.
        </p>
      </Section>
    </>
  )
}
