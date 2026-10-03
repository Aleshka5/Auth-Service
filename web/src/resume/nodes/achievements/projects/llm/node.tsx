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
        <strong>llm.filenkov.store</strong>, allows only a hub admin, and forwards to the llama.cpp
        server on an internal network. The only thing that server receives from the edge is a set of
        identity headers describing who is asking. The owl on this resume still talks to LiteLLM
        inside the network; this host is the model server itself.
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
