export default function StackNode() {
  return (
    <>
      <p className="lede">
        I think about my stack as three layers stacked in the order a request travels: an edge that decides who
        you are, a backend that holds the truth, and agents that do the actual work on top of both.
      </p>
      <p>
        The <strong>agent layer</strong> is what the job title is about. Graphs that plan, call tools, and write
        something a person would otherwise have written by hand — reports, routing decisions, extracted entities.
        This is where LangGraph, RAG and MCP live, and where the numbers in <em>Experience</em> come from.
      </p>
      <p>
        The <strong>backend layer</strong> is what makes an agent usable by anyone other than its author. A graph
        that only runs in a notebook is a demo. Put it behind FastAPI, give it Postgres for the things that must
        survive a restart, a queue for the work that should not block a request, and it becomes a service another
        team can call.
      </p>
      <p>
        The <strong>infra layer</strong> is the part most ML people skip and then pay for. One domain, TLS
        terminated once, identity injected at the edge so no downstream service reimplements auth, and shared
        object storage so files are not scattered across five containers. The site you are reading runs on it.
      </p>
      <p>
        The layers are not equally deep and I will not pretend otherwise. I am strongest at the top, competent and
        productive in the middle, and at the bottom I know enough to build and operate a small system end to end —
        which is exactly what this one is. What holds them together is that I have shipped through all three on
        the same project rather than learning each in isolation.
      </p>
    </>
  )
}
