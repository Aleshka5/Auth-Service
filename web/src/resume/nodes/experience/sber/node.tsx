import { Chips, Metric, Metrics } from '../../../../components/article'

export default function SberNode() {
  return (
    <>
      <p className="lede">
        Middle+ AI Agents. Reporting that used to occupy analysts runs on a graph instead — about a
        thousand reports a week, and roughly 1M ₽ a year that stopped being spent.
      </p>
      <Metrics>
        <Metric
          value="~1M ₽/yr"
          label="Saved on automated reporting"
          how="Measured against the manual cost of the same reporting volume: 1,000 reports a week."
        />
        <Metric
          value="1,000/week"
          label="Reports produced by the graph"
          how="Production volume, not a benchmark — the number the cost saving is derived from."
        />
      </Metrics>
      <p>
        The role was building and running the agent side of a reporting product. A report is not one
        model call: it is a plan, a set of tool calls that fetch and compute the underlying facts,
        retrieval over the domain so the wording stays grounded in the corpus rather than in the
        model’s prior, and a write-up step. I built those runs as explicit graphs in LangGraph —
        Plan-and-Execute and ReAct shapes — rather than as a single long prompt, because a graph is the
        thing you can measure, cache, and cut.
      </p>
      <p>
        Around the graph is ordinary backend work, and most of the reliability comes from there. FastAPI
        services expose the runs, PostgreSQL holds the state and the history, Kafka carries the
        asynchronous work so a slow report never blocks a request, and Faiss serves the vector search
        behind the RAG step. Tools were pulled out of the graph entirely into their own MCP service, so
        two product services share one tool layer.
      </p>
      <Chips items={['LangGraph', 'FastAPI', 'PostgreSQL', 'Kafka', 'Faiss', 'RAG']} />
      <p>
        Three pieces of that work are worth their own page: halving the token cost of a report, giving
        reports memory of each other, and moving the tools out to MCP.
      </p>
    </>
  )
}
