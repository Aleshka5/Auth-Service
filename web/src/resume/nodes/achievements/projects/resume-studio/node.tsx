import { Chips, Section } from '../../../../../components/article'

export default function ResumeStudioNode() {
  return (
    <>
      <p className="lede">
        Paste a vacancy into a chat and get back a resume and a cover letter written against it — as a
        typeset PDF, not as text in a box.
      </p>
      <p>
        The premise is that a good resume is not a document, it is a projection. The same career answers a
        platform-engineering ad and an applied-ML ad differently: different results lead, different
        systems get explained, different vocabulary. Doing that by hand is an hour per application and the
        quality decays as the pile grows, which is exactly the shape of problem worth handing to an agent.
      </p>
      <Chips items={['LangGraph', 'FastAPI', 'XeLaTeX']} />
      <Section title="Spec-driven pipeline">
        <p>
          The run goes knowledge base → agent graph → LaTeX artifact. The knowledge base holds the raw
          material — roles, projects, numbers, the measurement behind each one — and it is the only place
          facts live. The graph reads the vacancy, decides which of those facts the reader actually needs
          and in what order, and produces a specification of the document rather than the document itself.
          Rendering is a separate, deterministic step: XeLaTeX turns the spec into a PDF.
        </p>
        <p>
          Keeping generation and typesetting apart is the decision the whole thing rests on. A model that
          emits LaTeX directly will eventually emit LaTeX that does not compile, and debugging that means
          debugging prose. A model that emits a structured spec can be validated before anything is
          rendered, and the template owns the layout permanently.
        </p>
        <p>
          Humans stay in the loop on the weak cases. Where the graph is unsure which claim belongs in a
          document — or where the vacancy asks for something the knowledge base does not contain — it
          surfaces the choice instead of inventing an answer. A resume that quietly fabricates a number is
          worse than one that omits it.
        </p>
      </Section>
      <Section title="Files and identity">
        <p>
          Postgres holds the chats and the state of each run. MinIO holds the two kinds of file involved:
          the knowledge base documents going in, and the generated PDFs coming out — the same shared object
          store the other services on this domain use.
        </p>
        <p>
          Authentication is not implemented here at all. The studio sits behind the Auth Gateway like
          every other service, receives the <strong>X-Auth-*</strong> headers, and treats them as the
          signed-in user. Google sign-in happens once, at the hub, and this project inherits it.
        </p>
        <Chips items={['Postgres', 'MinIO', 'X-Auth-*']} />
      </Section>
    </>
  )
}
