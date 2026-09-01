import { Callout } from '../../../components/article'

export default function AchievementsNode() {
  return (
    <>
      <p className="lede">
        A result is something that changed for somebody else: money that stopped being spent, work that
        stopped needing a person, or a service that other people can now sign into.
      </p>
      <p>
        I keep the bar deliberately narrow here. A model that scores well on a validation split I built
        myself is not a result — it is a step. A rewrite that halves the token bill of a report the
        business already sends a thousand times a week is a result, because the saving continues after I
        stop looking at it. The same test applies to the services: a demo is not a result, a host that
        strangers can reach and that keeps working while I am asleep is.
      </p>
      <p>
        Every number on this branch travels with the sentence that explains how it was obtained. That
        second sentence is the part worth reading. A percentage without a dataset, a split, and a date is
        a claim about my optimism, not about the system. So each figure states what it was measured on —
        production volume, unseen mail, a token count taken before and after the same report — and where
        the boundary of the claim is.
      </p>
      <Callout title="What is not here">
        Numbers I cannot describe the measurement for. Some of the more interesting work at Sber and
        AeroClub sits behind internal reporting I cannot quote, so it is stated qualitatively or left out
        rather than rounded into a headline figure.
      </Callout>
      <p>
        The rest of the branch splits the two kinds of evidence: the outcomes I was paid to produce, and
        the systems I built and run myself, where the code, the domain, and the uptime are all mine and
        can be inspected directly.
      </p>
    </>
  )
}
