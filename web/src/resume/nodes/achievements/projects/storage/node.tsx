import { Chips, Section } from '../../../../../components/article'

export default function StorageNode() {
  return (
    <>
      <p className="lede">
        A home cloud for family files. The same Google sign-in as the rest of the domain, and permissions
        that stay on this host instead of following you around.
      </p>
      <p>
        Storage is a separate service on its own subdomain, and it does no authentication. The gateway
        matches the host, validates the access cookie, and proxies to an internal upstream with identity
        already attached, so the application code starts from a known user rather than from a token it
        has to interpret. Blobs live on the shared MinIO instance that also holds resume artifacts and
        service backups; Postgres holds the file and sharing metadata.
      </p>

      <Section title="Family-aware access">
        <p>
          The gateway treats this host specially. Alongside the usual X-Auth-* headers it looks the
          user&rsquo;s <strong>storage</strong> role up in User-Service and injects it as its own header,
          so what arrives is not the hub role but the role for this service only: ADMIN, FAMILY,
          STRANGER, or BLOCKED. A cookie is also forwarded so the per-session vault key reaches the
          storage backend.
        </p>
        <p>
          That separation is the point of the design. Family members get real access to a shared drive
          without becoming administrators of anything else, and a stranger who signs in to read the
          resume gets nothing here. If the directory is unreachable the gateway answers 503 and refuses
          to proxy rather than guessing a role — failing closed is the only safe default when the answer
          decides who can read somebody&rsquo;s files.
        </p>
        <Chips items={['FastAPI', 'Postgres', 'MinIO']} />
      </Section>
    </>
  )
}
