import { useCallback, useState } from 'react'
import { Link, Outlet } from 'react-router-dom'
import { useAuth } from '../auth'
import { useScrollChrome } from '../hooks/useScrollChrome'
import { isAdmin } from '../types'
import { Owl } from './owl/Owl'

function Mark() {
  return (
    <svg className="brand-mark" viewBox="0 0 32 32" aria-hidden="true">
      <path d="M16 3 L29 16 L16 29 L3 16 Z" fill="none" stroke="currentColor" strokeWidth="1.75" />
      <circle cx="16" cy="16" r="3.2" fill="var(--accent)" />
    </svg>
  )
}

/**
 * Service links (§7.1). Guests see LLM / Storage / Hermes with a lock rather
 * than not at all: the gateway redirects an unauthenticated navigation to
 * `/auth?return_to=…`, so clicking one is a working sign-in path. Admin is
 * hidden from non-admins, because advertising a 403 is noise.
 */
const services = [
  { label: 'LLM', href: 'https://llm.filenkov.store' },
  { label: 'Storage', href: 'https://storage.filenkov.store' },
  { label: 'Hermes', href: '/hermes/chat' },
] as const

export function Layout() {
  const { user, loading, logout } = useAuth()
  const [departing, setDeparting] = useState(false)
  const [departNonce, setDepartNonce] = useState(0)
  const admin = isAdmin(user)

  useScrollChrome()

  const finishDepart = useCallback(async () => {
    try {
      await logout()
    } finally {
      setDeparting(false)
    }
  }, [logout])

  return (
    <div className="shell">
      <a className="skip-link" href="#node-article">
        Skip to article
      </a>
      <header className="header">
        <Link className="brand" to="/">
          <Mark />
          <span className="brand-name">Filenkov</span>
        </Link>
        <nav className="service-nav" aria-label="Primary">
          {services.map((service) => (
            <a
              key={service.label}
              href={service.href}
              title={user ? undefined : 'Sign in required'}
            >
              {service.label}
              {user ? null : (
                <span className="service-lock" aria-hidden="true">
                  🔒
                </span>
              )}
            </a>
          ))}
          {admin ? <Link to="/admin">Admin</Link> : null}
        </nav>
        <div className="header-right">
          {loading ? (
            <span className="muted">Checking sign-in…</span>
          ) : user ? (
            <>
              <span className="user-name">{user.username}</span>
              <button
                type="button"
                className="btn btn-ghost btn-small"
                disabled={departing}
                onClick={() => {
                  setDeparting(true)
                  setDepartNonce((n) => n + 1)
                }}
              >
                Sign out
              </button>
            </>
          ) : (
            <Link className="btn btn-small" to="/auth">
              Sign in
            </Link>
          )}
        </div>
      </header>
      <Owl departing={departing} departNonce={departNonce} onDeparted={() => void finishDepart()} />
      <main className="main" id="main">
        <Outlet />
      </main>
      <footer className="footer">
        <span>Filenkov · personal tools</span>
        <span>Google sign-in · httpOnly cookies on .filenkov.store</span>
      </footer>
    </div>
  )
}
