import { profile } from '../profile'

export default function StartNode() {
  return (
    <>
      <p className="node-role">{profile.title}</p>
      <p className="lede">{profile.lede}</p>
      <dl className="a-deflist">
        {profile.contacts.map((contact) => (
          <div key={contact.label}>
            <dt>{contact.label}</dt>
            <dd>
              <a href={contact.href}>{contact.value}</a>
            </dd>
          </div>
        ))}
      </dl>
      <p className="muted">
        {profile.years} of experience · English {profile.english}
      </p>
    </>
  )
}
