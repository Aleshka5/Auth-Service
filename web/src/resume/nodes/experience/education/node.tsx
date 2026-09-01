import { Timeline } from '../../../../components/article'

export default function EducationNode() {
  return (
    <>
      <p className="lede">
        Two degrees, both taken alongside the jobs described on this branch. The engineering came first
        and the machine learning followed it, which is roughly the order the work happened in as well.
      </p>
      <Timeline
        items={[
          {
            when: '2024 — 2026',
            what: 'MAI (Moscow Aviation Institute) · Data Science and Machine Learning',
            detail: 'Dissertation: a Diffusion Transformer with short-term memory.',
          },
          {
            when: '2020 — 2024',
            what: "MPEI (National Research University 'MPEI') · Bachelor’s, Computer Systems and Engineering",
            detail: 'Thesis: a Telegram bot with a RAG system.',
          },
        ]}
      />
      <p>
        The MPEI degree is a systems degree rather than a modelling one — computer systems and
        engineering, which is the background the backend half of my work comes from. The thesis was the
        first thing I built that put a language model behind an interface people actually used: a Telegram
        bot answering from a document corpus through retrieval, which is the same RAG shape that later
        turned up in production reporting, only smaller and with worse retrieval.
      </p>
      <p>
        The MAI programme is the deliberate other half. I had been using models as they came and wanted to
        understand the machinery underneath rather than treat it as a service, so the dissertation went
        somewhere my day job does not: a Diffusion Transformer given short-term memory, so that generation
        can condition on what it produced recently instead of starting from the same state each step.
      </p>
      <p>
        I am not claiming research output from either. What the second degree gave me was the ability to
        read a paper about an architecture and know which parts of it are load-bearing — which is
        precisely the judgement needed when deciding whether a new model is worth switching to, or whether
        a problem is a prompt problem, a retrieval problem, or a graph problem.
      </p>
    </>
  )
}
