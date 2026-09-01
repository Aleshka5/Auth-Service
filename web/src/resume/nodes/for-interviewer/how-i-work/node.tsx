import { Section } from '../../../../components/article'

export default function HowIWorkNode() {
  return (
    <>
      <p className="lede">
        Every result elsewhere in this resume came out of roughly the same loop: find the number the business
        actually cares about, get the cheapest version of the thing in front of a human fast, then spend the real
        effort on the gap between demo and dependable.
      </p>

      <Section title="From an ask to a shipped agent">
        <p>
          A business ask arrives as an outcome, not a specification — “analysts spend a day a week on these
          reports”, “this mailbox needs routing”. My first job is to turn it into something measurable, because
          without that there is no way to tell whether a change helped. At AeroClub the measurable thing was
          precision and recall on unseen mail; at Sber it was tokens per report and the analyst hours behind a
          thousand reports a week.
        </p>
        <p>
          Then the cheapest honest version: prompts against real examples, no infrastructure, no fine-tune. That
          answers the question that actually matters — is the task well-defined enough for a model at all? Most of
          the failures I have seen come from skipping this and discovering three weeks in that two humans label
          the same email differently.
        </p>
        <p>
          Only once the shape holds does it become a service: a graph instead of a script, tools moved out so they
          are shared rather than copied, a queue in front of the slow parts, and a way to inspect a single run
          after the fact. That last one is not optional. The 2× token reduction was possible because I could read
          what the graph did on a real report, not because I guessed at a better prompt.
        </p>
      </Section>

      <Section title="Prompt engineering, fine-tuning, or plain code">
        <p>
          I default to plain code for anything deterministic. If a rule can be written as a rule, a model is a
          slower and less predictable way to write it, and it will be the part that breaks at 3 a.m.
        </p>
        <p>
          Prompt engineering is where I start for language work, and often where I stop — the production scores on
          mail routing and NER at AeroClub were prompt-only. It is fast to iterate, easy to explain to a
          non-engineer, and it fails visibly. Fine-tuning earns its place when prompts have plateaued on a
          specific, repeated failure and there is labelled data to fix it with. It is a commitment: data
          pipeline, evaluation, retraining, and a model to keep alive. I want to be sure the ceiling is real
          before paying for it.
        </p>
      </Section>

      <Section title="Review, in both directions">
        <p>
          I would rather send a small change that is easy to disagree with than a large one that is easy to wave
          through. When I review, I look first at what happens when the model is wrong, because that path is the
          one nobody tested. For agent work I ask to see a trace of an actual run — reading the graph is not the
          same as reading what it did.
        </p>
      </Section>

      <Section title="What I need from a team">
        <p>
          Three things, and they are the same three every time. One person who owns the metric and can say
          whether it moved. Access to real data early — synthetic examples hide exactly the messiness that breaks
          extraction. And someone with the standing to say no, so that scope stays small enough to actually ship
          and a good result is not diluted into four half-finished ones.
        </p>
      </Section>
    </>
  )
}
