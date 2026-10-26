import { useCallback, useState } from 'react'
import { Link, Outlet } from 'react-router-dom'
import { useAuth } from '../auth'
import { useReadyModels } from '../hooks/useReadyModels'
import { useScrollChrome } from '../hooks/useScrollChrome'
import { isAdmin, serviceRole } from '../types'
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
 * Service links (§7.1). Guests see Storage / Hermes / RAG with a lock rather
 * than not at all: the gateway redirects an unauthenticated navigation to
 * `/auth?return_to=…`, so clicking one is a working sign-in path. Admin and
 * Chat are hidden unless hub `global_role` is ADMIN, because advertising a
 * 403 is noise. Image is shown only for `image` FAMILY or ADMIN, never locked.
 * Chat and Image also stay hidden unless Switch-Service reports that model
 * ready (`llm` and `image`).
 */
const services = [
  { label: 'Storage', href: 'https://storage.filenkov.store' },
  { label: 'Hermes', href: 'https://hermes.filenkov.store' },
  { label: 'RAG', href: 'https://rag.filenkov.store' },
] as const

export function Layout() {
  const { user, loading, logout } = useAuth()
  const [departing, setDeparting] = useState(false)
  const [departNonce, setDepartNonce] = useState(0)
  const admin = isAdmin(user)
  const imageRole = user ? serviceRole(user, 'image') : null
  const imageAllowed = imageRole === 'FAMILY' || imageRole === 'ADMIN'
  const models = useReadyModels(admin || imageAllowed)

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
        <nav className="service-nav" aria-label="Primary" data-model-lookup={models.status}>
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
          {imageAllowed && models.image ? <a href="https://image.filenkov.store">Image</a> : null}
          {admin && models.llm ? <a href="https://llm.filenkov.store">Chat</a> : null}
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
