import { Link, useSearchParams } from 'react-router-dom'
import { googleAuthPath } from '../api'
import { useAuth } from '../auth'

function GoogleMark() {
  return (
    <svg className="google-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="currentColor"
        d="M21.35 11.1h-9.18v2.96h5.27c-.23 1.22-1.37 3.58-5.27 3.58-3.17 0-5.76-2.62-5.76-5.84s2.59-5.84 5.76-5.84c1.8 0 3.01.77 3.7 1.43l2.52-2.43C16.54 3.54 14.5 2.6 12.17 2.6 7.03 2.6 2.87 6.76 2.87 11.9s4.16 9.3 9.3 9.3c5.37 0 8.92-3.77 8.92-9.08 0-.61-.07-1.07-.16-1.52Z"
      />
    </svg>
  )
}

function sameOriginReturnTo(raw: string | null): string | null {
  if (!raw) return null
  try {
    const u = new URL(raw, window.location.origin)
    if (u.username || u.password) return null
    if (u.origin !== window.location.origin) return null
    return `${u.pathname}${u.search}${u.hash}` || '/'
  } catch {
    return null
  }
}

export function Auth() {
  const { user, loading } = useAuth()
  const [params] = useSearchParams()
  const returnTo = params.get('return_to')
  const href = googleAuthPath(returnTo)
  const continueHref = sameOriginReturnTo(returnTo)

  return (
    <section className="auth-page">
      <div className="panel stack">
        <p className="kicker">Sign in</p>
        <h1 style={{ fontSize: '2.1rem' }}>Google only.</h1>
        <p className="muted">
          This hub does not keep passwords. Continue with the Google account you want on Filenkov
          services. Sign-in sets httpOnly access and refresh cookies — the page never sees the JWTs.
        </p>
        {loading ? (
          <p className="muted">Checking sign-in…</p>
        ) : user ? (
          <>
            <p>
              You are already signed in as <strong>{user.username}</strong>.
            </p>
            {continueHref ? (
              <a className="btn" href={continueHref}>
                Continue to destination
              </a>
            ) : returnTo ? (
              <a className="btn" href={href}>
                Continue to destination
              </a>
            ) : null}
            <Link className="btn btn-ghost" to="/">
              Back home
            </Link>
          </>
        ) : (
          <a className="btn" href={href}>
            <GoogleMark />
            Continue with Google
          </a>
        )}
      </div>
    </section>
  )
}
