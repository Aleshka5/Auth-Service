import { Callout, Section } from '../../../../components/article'

export default function WhyATreeNode() {
  return (
    <>
      <p className="lede">
        A resume that scrolls gives every reader the same words in the same order. That is one ordering for at least
        three different people, so it is the right ordering for none of them.
      </p>

      <p>
        The old version of this site printed everything on one page — visit card, stack, projects, experience, the
        architecture map — with accordions that expanded in place. It worked, and it only ever grew. A recruiter had to
        scroll past LangGraph internals to find out whether I am available; an engineer had to scroll past availability
        to find out how the token cost came down. Every addition made the page worse for someone.
      </p>

      <p>
        So the page became a tree you walk. The root is a visit card and four branches. A recruiter reaches{' '}
        <strong>For interviewer</strong> in one click. An engineer reaches <strong>2× cheaper tokens</strong> in three,
        through Experience and Sber, and reads a single article about that one thing. Nobody scrolls past what they did
        not ask for, and every node has its own URL, so I can send exactly the relevant one into a conversation.
      </p>

      <Section title="The four rules">
        <ol>
          <li>
            <strong>Minimalism</strong> — the feel of a paper article. One column, one raised surface, no card grids
            competing for attention.
          </li>
          <li>
            <strong>A tree you walk</strong> — root at the top, the reader chooses the next stage, the spine says where
            they are.
          </li>
          <li>
            <strong>Nodes are trivial to add</strong> — a node is a folder with two files, and its folder path is its
            URL. There is no list to keep in sync anywhere.
          </li>
          <li>
            <strong>Content loads on demand</strong> — only the metadata for every node ships in the main bundle; an
            article is fetched when you open it, and prefetched when you look like you are about to.
          </li>
        </ol>
      </Section>

      <Section title="What rule 3 costs">
        <p>
          Convention over configuration is what makes adding a node cheap, and it has one price: a typo is not a compile
          error. Misspell a folder, forget the <code>node.tsx</code> beside a <code>meta.ts</code>, give two siblings the
          same sort order, and you do not get a red build — you get a node that quietly is not there, or an order that
          silently flips.
        </p>
        <p>
          That is why the registry has a test rather than a schema. It walks everything the glob discovers and fails the
          build on an orphan file, a slug that is not lowercase kebab-case, a reserved first segment, a duplicate order
          among siblings, a missing parent, or a path deeper than four levels. The convention stays free to use, and the
          test buys back the compile error it gave up.
        </p>
      </Section>

      <Callout>
        <p>
          The test is the honest version of this design. Any structure that is inferred rather than declared needs
          something that checks the inference — otherwise the convenience is borrowed against a future outage.
        </p>
      </Callout>
    </>
  )
}
