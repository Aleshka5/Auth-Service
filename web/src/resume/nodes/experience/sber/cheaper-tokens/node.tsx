import { Metric, Metrics, Section } from '../../../../../components/article'

export default function CheaperTokensNode() {
  return (
    <>
      <p className="lede">
        A single report cost about 60,000 tokens. After rewriting the graph it cost about 30,000, and
        the reports it produced were the same ones.
      </p>
      <Metrics>
        <Metric
          value="60k → 30k"
          label="Tokens per report"
          how="Same report, same inputs, token count taken before and after the rewrite."
        />
      </Metrics>
      <p>
        At a thousand reports a week, per-run token spend stops being a detail and becomes the operating
        cost of the product. The first version of the report graph worked, which is why it shipped, but
        it was expensive in a way that was invisible until someone counted. So the first step was not
        optimisation at all: it was instrumenting the run so that every node in the graph reported what
        it had consumed. Once the spend was attributed per step rather than per report, the expensive
        parts were obvious, and they were not the ones I would have guessed.
      </p>
      <p>
        Most of the waste was repetition. The model was being handed the same context again on each
        pass, and it was re-deriving the same intermediate conclusions because nothing in the run
        carried them forward. That is a graph problem, not a prompt problem — a longer prompt would not
        have fixed it, and a bigger model would only have made it cost more.
      </p>
      <Section title="Fewer loops, same quality">
        <p>
          The rewrite tightened the Plan-and-Execute shape: fewer passes through the loop, an explicit
          plan that the executor follows instead of re-planning, and state carried between steps so a
          fact retrieved once is not retrieved again. Tools moved out of the graph into a separate
          service, which removed the tool descriptions and their re-discovery from the model’s context
          on every iteration.
        </p>
        <p>
          The constraint on all of it was that the output had to stay the same. Halving the token bill
          by producing a thinner report would not have been a saving, so the comparison was always the
          same report, before and after — and it was the shape of the run, not the quality of the
          answer, that got smaller.
        </p>
      </Section>
    </>
  )
}
