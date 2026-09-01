import { Chips, Metric, Metrics } from '../../../../../components/article'

export default function NerNode() {
  return (
    <>
      <p className="lede">
        Routing tells you where a message goes. Extraction tells you what is in it — and downstream
        systems need fields, not paragraphs. On mail the prompts had never seen, entity extraction ran
        above 80%.
      </p>
      <Metrics>
        <Metric
          value="> 80%"
          label="Entity extraction on new mail"
          how="Evaluated against mail that arrived after the prompts were written, with no fine-tune, then served in production behind FastAPI."
        />
      </Metrics>
      <p>
        The entities were the ones the business actually acts on: names, dates, routes, reference numbers,
        the identifiers that connect a message to a record in another system. Free-form correspondence is
        an unfriendly place to look for them. The same date appears written six ways, a reference number
        may be quoted inside a forwarded thread, and Russian inflection means the same name is spelled
        differently depending on where in the sentence it lands — which is what PyMorphy was there for,
        normalising forms so a match downstream did not depend on a grammatical case.
      </p>
      <p>
        As with routing, this shipped prompt-only. The extractor asks the model for a structured object
        with a fixed set of keys and validates the result before anything else sees it, so a malformed or
        hallucinated field fails at the boundary rather than propagating into a record. Anything that
        fails validation, or that the model is not confident about, is not silently dropped — it goes to
        the human-in-the-loop path described on the next page.
      </p>
      <Chips items={['NER', 'PyMorphy', 'FastAPI']} />
      <p>
        Serving it was deliberately unremarkable: a FastAPI endpoint that takes a message and returns the
        extracted fields. Keeping the model behind an ordinary HTTP contract meant the systems consuming
        the output never had to know that a language model was involved, and it left the door open to
        swap the implementation later without renegotiating anything with the callers.
      </p>
    </>
  )
}
