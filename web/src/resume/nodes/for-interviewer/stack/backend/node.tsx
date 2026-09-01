import { Chips, Section } from '../../../../../components/article'

export default function BackendNode() {
  return (
    <>
      <p className="lede">
        APIs other services can trust: identity, roles, and queues. Python and FastAPI for the services that carry
        model work, Go where the thing has to be small, fast and boring — the auth gateway in front of this site
        is Go for exactly that reason.
      </p>
      <p>
        The test I apply to a backend I have written is whether another team could adopt it without reading my
        code. That means the contract is explicit, the failure modes are documented, and the service says no
        clearly instead of half-succeeding. An agent behind a vague API is worse than no agent, because someone
        downstream will build on a behaviour I never promised.
      </p>
      <Chips items={['Python', 'FastAPI', 'Go']} />

      <Section title="Data & buses">
        <p>
          Postgres for truth — anything that must survive a restart or be reconciled later. Redis for the things
          that are fast, small and allowed to be lost: rejected tokens, OAuth state, rate-limit counters. Kafka or
          RabbitMQ when the work is genuinely async, which for agent workloads it usually is; a report that takes
          ninety seconds has no business holding an HTTP request open.
        </p>
        <Chips items={['PostgreSQL', 'Redis', 'Kafka', 'RabbitMQ']} />
      </Section>

      <Section title="Auth contracts">
        <p>
          Google OAuth2 as the only credential, httpOnly access and refresh cookies scoped to the{' '}
          <strong>.filenkov.store</strong> domain, and an Auth Gateway that verifies the token once and injects{' '}
          <strong>X-Auth-*</strong> headers into every proxied request. Downstream services never see a JWT and never
          reimplement verification — they read a header and trust the edge. That single decision is what let me
          add a storage service, an LLM workspace and this resume to the same domain without writing auth three
          more times.
        </p>
        <Chips items={['OAuth2', 'JWT cookies']} />
      </Section>
    </>
  )
}
