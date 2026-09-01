import { Chips, Metric, Metrics } from '../../../../components/article'

export default function AeroClubNode() {
  return (
    <>
      <p className="lede">
        Middle LLM Engineer. A travel agency’s incoming mail used to be read and routed by people; by the
        time I left, that routing ran itself at 85% precision and 80% recall on mail nobody had seen.
      </p>
      <Metrics>
        <Metric
          value="85% / 80%"
          label="Precision / recall on unseen mail"
          how="Scored on incoming mail that arrived after the prompts were written — not a resampled training split."
        />
      </Metrics>
      <p>
        The problem is easy to state and unpleasant to solve. Mail arrives continuously, in free form,
        from customers and from suppliers, and before anyone can act on it somebody has to decide what it
        is about and who should get it. One message often belongs to several categories at once, which
        makes it a multi-label problem rather than a choice between buckets, and the categories are the
        business’s own, so no off-the-shelf classifier knows them.
      </p>
      <p>
        My side of it was the classification and extraction layer and the machinery around it. Routing
        decided where a message went; named-entity extraction pulled the facts out of it so downstream
        systems received fields rather than prose. Both shipped prompt-first — the production scores were
        reached without a fine-tune — which mattered because the categories kept moving and a prompt can
        follow a moving definition in an afternoon.
      </p>
      <Chips items={['LangChain', 'FastAPI', 'LabelStudio', 'RabbitMQ', 'scikit-learn']} />
      <p>
        Around the models is ordinary service work. FastAPI exposes the classifier and the extractor,
        RabbitMQ carries the asynchronous loop so a slow model call never blocks an incoming message, and
        LabelStudio is where the cases the system is not confident about go to be decided by a person.
        scikit-learn was not used for the production classifier; it was used to score it, which turned out
        to be the more valuable half.
      </p>
      <p>
        Three parts of this are worth their own page: the routing itself and how it was measured,
        the entity extraction, and the human-in-the-loop framework that made both of them improvable.
      </p>
    </>
  )
}
