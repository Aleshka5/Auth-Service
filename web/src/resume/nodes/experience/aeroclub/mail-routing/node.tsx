import { Chips, Section } from '../../../../../components/article'

export default function MailRoutingNode() {
  return (
    <>
      <p className="lede">
        Incoming mail sorted into overlapping business categories by Mistral Large, driven by prompts
        alone. The production numbers — 85% precision, 80% recall on new mail — were reached without a
        fine-tune.
      </p>
      <p>
        Multi-label is the part that makes this harder than it sounds. A single message can be a booking
        change, a complaint, and a payment question at once, so the system cannot pick a winner; it has to
        decide independently, for each category, whether the message belongs to it. That also means the
        obvious metric is useless. Accuracy over a set of labels that are mostly absent looks excellent
        while the system is quietly failing on the labels anyone cares about.
      </p>
      <p>
        Going prompt-first was a decision about change, not about effort. The categories were the
        business’s, and the business kept refining them: a class would be split, or a new supplier would
        introduce a kind of message that had not existed the month before. A fine-tuned model answers
        that with a labelling round and a retraining cycle. A prompt answers it the same day, and when the
        prompt-only scores turned out to be good enough for production, there was never a reason to spend
        the training budget.
      </p>
      <Chips items={['Mistral Large', 'LangChain', 'FastAPI']} />
      <Section title="Task-shaped confusion matrix">
        <p>
          The evaluation was built around the shape of the task rather than around a single score. Instead
          of one number per run, every label got its own confusion matrix, and the matrices were read
          together: which classes the model over-claimed, which ones it missed, and — the useful part —
          which pairs of classes it confused with each other.
        </p>
        <p>
          That is what made the floating false-positive classes visible. A handful of categories were
          absorbing messages that belonged elsewhere, and because they were rare the aggregate metrics
          never moved enough to point at them. With the matrix in front of me, prompt iterations went
          after the weak labels first instead of being spread evenly across a definition that was already
          working.
        </p>
        <p>
          scikit-learn produced the matrices; there is no machine learning of my own in the pipeline. The
          scoring code was the smallest part of the work and by some distance the highest-leverage.
        </p>
      </Section>
    </>
  )
}
