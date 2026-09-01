import { Chips, Section } from '../../../../../components/article'

export default function AiAgentsNode() {
  return (
    <>
      <p className="lede">
        Agents that route work, extract facts, and write reports without a human in every loop. This is the layer
        I am hired for, and the one where a wrong structural decision costs real money in tokens.
      </p>
      <p>
        The pattern I reach for is a graph, not a chat loop. A graph makes the run inspectable: you can see which
        node planned, which node called a tool, and where the model went round in circles. That visibility is what
        made the cost work at Sber possible — you cannot halve a token bill you cannot read.
      </p>
      <Chips items={['LangChain', 'LangGraph', 'RAG', 'Faiss']} />

      <Section title="Plan-and-Execute & ReAct">
        <p>
          Graph-shaped runs: plan first, then tool calls, with memory carried between reports. Plan-and-Execute
          when the work decomposes cleanly and the plan is worth spending tokens on; ReAct when the next step
          genuinely depends on what the last tool returned. Most production graphs I have written are the first
          with a small amount of the second inside a single step.
        </p>
      </Section>

      <Section title="MCP tools">
        <p>
          Tools live in a separate Model Context Protocol service so the graphs stay thin and reusable. At Sber
          this meant two product services shared one tool layer instead of each carrying its own copy — and the
          model stopped re-discovering tool definitions on every run, which was part of the token saving.
        </p>
        <Chips items={['MCP', 'FastAPI']} />
      </Section>

      <Section title="RAG & embeddings">
        <p>
          Domain retrieval so the model answers from the corpus rather than from hope. Faiss for the index,
          HuggingFace encoders, PyTorch where a model needs touching directly. The engineering that matters here
          is rarely the vector store; it is what gets chunked, what gets filtered out before retrieval, and how
          you tell whether a retrieved passage actually helped.
        </p>
        <Chips items={['Faiss', 'HuggingFace', 'PyTorch']} />
      </Section>

      <Section title="Prompt-first extraction">
        <p>
          Multi-label mail routing and NER at AeroClub shipped on prompt engineering, with no fine-tune behind the
          production scores. Prompt-first is not a stylistic preference — it is the cheapest way to find out
          whether the task is even well-defined before anyone pays for labelled data.
        </p>
        <Chips items={['Mistral Large', 'NER', 'PyMorphy']} />
      </Section>
    </>
  )
}
