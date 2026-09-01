import { DefinitionList, Section } from '../../../../components/article'
import { profile } from '../../../profile'

export default function LogisticsNode() {
  return (
    <>
      <p className="lede">
        The screening facts, in one place, so you do not have to read an article to find them.
      </p>
      <DefinitionList
        items={[
          { term: 'Role', detail: profile.title },
          { term: 'Experience', detail: profile.years },
          { term: 'English', detail: profile.english },
          ...profile.contacts.map((contact) => ({
            term: contact.label,
            detail: <a href={contact.href}>{contact.value}</a>,
          })),
        ]}
      />
      <p>
        Two of those are worth a sentence rather than a line. The five years are continuous and all of
        them are language-model work in production — 2021 to 2023 at AeroClub on mail classification and
        extraction, 2023 to 2026 at Sber on reporting agents. There is no gap to explain and no career
        change to account for; the <em>Experience</em> branch is the same period told at more length.
      </p>
      <p>
        English at B1 means I read documentation, specifications, papers, and code review comments without
        friction, and I write them. Speaking is the weaker half: I can hold a technical conversation, and I
        will be slower than a native speaker in an unstructured one. I would rather you know that before
        the call than discover it during one.
      </p>
      <Section title="Location, notice, and format">
        <p>
          These are the questions I will not answer with a number on a web page, because the honest answer
          depends on the offer. Notice period, start date, on-site versus remote, and where I would be
          working from are all things I would rather settle in the first call, in the context of the actual
          role, than have you match against a figure I wrote months ago.
        </p>
        <p>
          Ask me directly and you will get a straight answer the same day. Telegram is the fastest of the
          three addresses above; email is the one to use if you need a written record.
        </p>
      </Section>
      <p>
        If you need something in a specific format for your own process — a filled-in template, a
        structured summary, references — say which format and I will send it. There is no download here on
        purpose: a stale file is worse than none, and I would rather write you the current one.
      </p>
    </>
  )
}
