import { Chips } from '../../../../../components/article'

export default function ReportMemoryNode() {
  return (
    <>
      <p className="lede">
        A report generated in isolation is correct on its own and wrong next to last week’s. Memory
        between runs, and retrieval over the domain, are what keep a series of reports agreeing with
        itself.
      </p>
      <p>
        The failure mode is subtle and it does not show up in a single-run evaluation. Each report reads
        well; put two consecutive ones side by side and the same entity is named two ways, a figure is
        framed against a different baseline, and a caveat that was stated last time has quietly
        disappeared. A reader who follows the series notices immediately, and once they notice, they
        stop trusting the numbers as well as the wording.
      </p>
      <p>
        Two mechanisms address it. The first is memory: a run is linked to the runs before it, so the
        graph can see what was already said about the same subject and stay consistent with it rather
        than starting from a blank context. The second is retrieval — a Faiss index over the domain
        corpus, so terminology, definitions, and standing context come from the corpus instead of being
        reinvented per run. Between them, the model is answering from something durable rather than from
        whatever happened to be in the prompt that day.
      </p>
      <Chips items={['Memory', 'Faiss', 'RAG']} />
      <p>
        This also pays for itself on cost. Context that is retrieved on demand does not have to be
        pasted into every prompt, and a conclusion that is remembered does not have to be re-derived —
        the same pressure that drove the token work on the neighbouring branch.
      </p>
    </>
  )
}
