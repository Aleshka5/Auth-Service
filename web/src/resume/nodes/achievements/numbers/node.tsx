import { Metric, Metrics, Section } from '../../../../components/article'

export default function NumbersNode() {
  return (
    <>
      <p className="lede">
        Four figures I am willing to defend line by line. Each one names the system it came from and the
        conditions it was taken under.
      </p>
      <Metrics>
        <Metric
          value="~1M ₽/year"
          label="Reporting cost removed at Sber"
          how="Measured against the running volume of 1,000 reports a week: the analyst time and model spend the automated pipeline replaced, annualised."
        />
        <Metric
          value="60k → 30k"
          label="Tokens per report at Sber"
          how="Same report, same inputs, counted before and after the graph rewrite — a straight halving of the per-run token bill, not an average over a changed workload."
        />
        <Metric
          value="85% / 80%"
          label="Precision / recall, multi-label mail routing at AeroClub"
          how="Scored on unseen incoming mail, not a resampled training split, and reached prompt-only on Mistral Large with no fine-tune."
        />
        <Metric
          value="> 80%"
          label="NER on new mail at AeroClub"
          how="Entity extraction evaluated on mail the prompt had never seen, again without a fine-tune, then shipped behind FastAPI."
        />
      </Metrics>
      <Section title="Why the measurement matters more than the value">
        <p>
          The two Sber numbers are linked but independent. The cost figure is an operational one: it only
          holds because the volume is real and sustained, so halving the token count of a single report
          compounds across a thousand runs a week. The token figure is a controlled comparison — the same
          report, the same source data, run through the old and the new graph — which is why it can be
          stated as an exact ratio rather than a range.
        </p>
        <p>
          The two AeroClub numbers share one condition worth stating loudly: <strong>unseen data</strong>.
          Both were scored on mail that arrived after the prompts were written. A confusion matrix shaped
          around the actual task, rather than a flat accuracy score, is what made the weak labels visible
          and let prompt iterations go after them first.
        </p>
        <p>
          Nothing here required a fine-tune. That is a deliberate part of the result: the routing and
          extraction scores were reached with prompt engineering and evaluation discipline alone, which
          kept the systems cheap to change when the mail did.
        </p>
      </Section>
    </>
  )
}
