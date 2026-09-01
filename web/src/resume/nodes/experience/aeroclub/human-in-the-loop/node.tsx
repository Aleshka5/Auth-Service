import { Chips, Section } from '../../../../../components/article'

export default function HumanInTheLoopNode() {
  return (
    <>
      <p className="lede">
        Neither the routing nor the extraction was ever going to be right every time. What made them
        usable in production was having somewhere for the cases they got wrong to go, and a path back
        from there into the next version.
      </p>
      <p>
        The agent logic lives in one framework rather than in per-task scripts. Routing and extraction are
        different jobs, but the surrounding machinery is the same in both: build the prompt, call the
        model, parse and validate the result, decide whether the answer is good enough to act on, and
        record what happened. Writing that once and parameterising the differences meant a new task
        started as a configuration rather than as a new pipeline, and — more usefully — that a fix to the
        validation or the logging applied everywhere at once.
      </p>
      <p>
        The decision that matters is the confidence gate. Every result is checked before it is trusted:
        the structure has to parse, the fields have to be plausible, and the model’s own hedging is taken
        seriously. What passes goes straight into the business flow. What does not is routed to a person,
        which is where LabelStudio comes in — the uncertain cases become annotation tasks with the model’s
        proposal already filled in, so a human is correcting a draft rather than starting from a blank
        message.
      </p>
      <Chips items={['LabelStudio', 'RabbitMQ']} />
      <Section title="Why the loop is asynchronous">
        <p>
          Mail arrives when it arrives, and a model call takes as long as it takes. Coupling those
          directly would mean an ingest endpoint that blocks on a language model, which fails badly the
          first time the model is slow. RabbitMQ decouples them: messages are queued, workers consume at
          their own rate, review tasks and their answers travel on the same bus, and a backlog becomes a
          delay rather than an outage.
        </p>
        <p>
          The other reason is that the loop has to close. Corrections coming out of review are not just
          fixes to individual messages — they are the record of where the prompts are wrong. Fed back into
          the evaluation set, they are what the confusion matrices are computed over, and they are how the
          weak labels were found and rewritten rather than guessed at.
        </p>
      </Section>
    </>
  )
}
