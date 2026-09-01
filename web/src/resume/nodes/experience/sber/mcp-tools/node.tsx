import { Chips, Section } from '../../../../../components/article'

export default function McpToolsNode() {
  return (
    <>
      <p className="lede">
        The tools an agent calls stopped living inside the agent. They moved into their own Model Context
        Protocol service, and two product services now share one tool layer.
      </p>
      <p>
        A reporting graph needs to fetch numbers, query the domain, run computations, and format results.
        In the first version each of those was a function registered inside the graph, which is the
        natural way to start and the reason the second product service copied them. Two copies of a tool
        is two places to fix a query, two sets of credentials, and two chances for the same tool to
        behave differently depending on which graph called it.
      </p>
      <p>
        MCP gave the split a shape I did not have to invent. The tools sit behind a FastAPI service that
        speaks the protocol: it advertises what it can do, takes a call, and returns a result. The graphs
        no longer own tool code at all — they own a plan and a client. Adding a tool is a deployment of
        one service, and it becomes available to every graph at once without a release on the product
        side.
      </p>
      <Chips items={['MCP', 'FastAPI']} />
      <Section title="What the separation actually bought">
        <p>
          The first gain was context size. Tool definitions travel in the model’s context, and when tools
          are scattered through a graph they tend to be re-declared on every pass. With one service
          holding the catalogue, the definitions are stated once per run rather than per iteration —
          which is a meaningful part of the token reduction described on the previous page.
        </p>
        <p>
          The second gain was testing. A tool service is an ordinary HTTP service with ordinary inputs and
          outputs, so it can be tested without invoking a model at all. When a report came out wrong, the
          question <em>did the tool return the wrong number, or did the model use the right number
          badly?</em> became answerable by calling the tool directly, instead of by re-reading a trace.
        </p>
        <p>
          The cost is a network hop and one more service to run. At the volumes involved, that was the
          cheaper side of the trade by a wide margin.
        </p>
      </Section>
    </>
  )
}
