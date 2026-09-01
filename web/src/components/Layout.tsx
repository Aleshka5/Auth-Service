import { useCallback, useState } from 'react'
import { Link, NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../auth'
import { isAdmin } from '../types'
import { HeaderMascot } from './HeaderMascot'

function Mark() {
  return (
    <svg className="brand-mark" viewBox="0 0 32 32" aria-hidden="true">
      <path d="M16 3 L29 16 L16 29 L3 16 Z" fill="none" stroke="currentColor" strokeWidth="1.75" />
      <circle cx="16" cy="16" r="3.2" fill="var(--copper)" />
    </svg>
  )
}

export function Layout() {
  const { user, loading, logout } = useAuth()
  const [departing, setDeparting] = useState(false)
  const [departNonce, setDepartNonce] = useState(0)
  const admin = isAdmin(user)

  const finishDepart = useCallback(async () => {
    try {
      await logout()
    } finally {
      setDeparting(false)
    }
  }, [logout])

  return (
    <div className="shell">
      <header className="header">
        <div className="header-left">
          <Link className="brand" to="/">
            <Mark />
            <span className="brand-name">Filenkov</span>
          </Link>
          <nav className="nav" aria-label="Primary">
            <NavLink to="/" end>
              Home
            </NavLink>
            {admin ? <NavLink to="/admin">Admin</NavLink> : null}
            {user ? <a href="/hermes/chat">Hermes</a> : null}
          </nav>
        </div>
        <HeaderMascot
          departing={departing}
          departNonce={departNonce}
          onDeparted={() => void finishDepart()}
        />
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
      <main className="main">
        <Outlet />
      </main>
      <footer className="footer">
        <span>Filenkov · personal tools</span>
        <span>Google sign-in · httpOnly cookies on .filenkov.store</span>
      </footer>
    </div>
  )
}
