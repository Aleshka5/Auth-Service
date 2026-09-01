export default function ForInterviewerNode() {
  return (
    <>
      <p className="lede">
        The rest of this resume is organised the way I would tell the story out loud — result first, then the
        implementation underneath it. This branch is the other order: the things you probably need before you
        decide whether the story is worth an hour of your time.
      </p>
      <p>
        If you are screening, everything you would normally ask in the first five minutes is here — years,
        English, how to reach me, what happens next. None of it is buried in prose; it is a list, and the list is
        short on purpose.
      </p>
      <p>
        If you are the engineer who will actually work with me, the interesting part is the technology I have my
        hands on and the way I get from a business ask to something running. I have tried to write both honestly:
        the tools I use weekly are named, the ones I have touched once are not, and the method I describe is the
        one that produced the numbers in <em>Experience</em>, not a framework I read about.
      </p>
      <p>
        A note on how to read the technology pages. I do not list a language and stop there. Each layer says what
        it is for in the systems I have built — agents that route work and write reports, an API other services
        can trust with identity, and an edge that puts all of it on one domain. The chips are there so you can
        scan, and the sentences are there because a chip list on its own tells you nothing about whether someone
        can ship.
      </p>
      <p>
        Anything below that is not concrete enough for you, ask directly. I would rather answer a sharp question
        than have you infer it.
      </p>
    </>
  )
}
