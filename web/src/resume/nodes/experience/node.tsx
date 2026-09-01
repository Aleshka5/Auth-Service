import { Timeline } from '../../../components/article'

export default function ExperienceNode() {
  return (
    <>
      <p className="lede">
        Five years, two companies, and one continuous thread: putting a language model somewhere a
        business process used to need a person, and then making that placement cheap enough to keep.
      </p>
      <p>
        The work started at AeroClub in 2021, on incoming mail. A travel agency receives a large volume
        of correspondence that has to be read, classified, and routed before anyone can act on it. I
        built the classification and extraction side of that — multi-label routing and named-entity
        extraction — and, just as importantly, the loop around it: a place for the hard cases to go, a
        queue to carry them, and an evaluation shape that told me which labels were actually failing.
      </p>
      <p>
        At Sber from 2023 the unit of work changed from a single message to a whole report. Instead of
        one classification call there is a graph: a plan, a sequence of tool calls, retrieval over a
        domain corpus, and memory that carries context from one report to the next. Running that a
        thousand times a week makes the cost of a single run a real engineering constraint, so a large
        part of the job was measuring token spend and cutting it without losing quality.
      </p>
      <Timeline
        items={[
          {
            when: '2021 — 2023',
            what: 'AeroClub · Middle LLM Engineer',
            detail: 'Mail routing and entity extraction in production, prompt-first, with a human-in-the-loop framework behind it.',
          },
          {
            when: '2023 — 2026',
            what: 'Sber · Middle+ AI Agents',
            detail: 'Automated reporting on agent graphs: Plan-and-Execute, RAG over the domain, tools split into their own MCP service.',
          },
        ]}
      />
      <p>
        Both roles were closer to backend engineering than to research. The models were used as they
        came; what I built around them — the graphs, the services, the queues, the databases, and the
        measurements — is where the results came from.
      </p>
    </>
  )
}
