import { Link, NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../auth'
import { isAdmin } from '../types'

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
  const admin = isAdmin(user)

  return (
    <div className="shell">
      <header className="header">
        <Link className="brand" to="/">
          <Mark />
          <span className="brand-name">Filenkov</span>
        </Link>
        <nav className="nav" aria-label="Primary">
          <NavLink to="/" end>
            Home
          </NavLink>
          {admin ? <NavLink to="/admin">Admin</NavLink> : null}
        </nav>
        <div className="header-right">
          {loading ? (
            <span className="muted">Checking session…</span>
          ) : user ? (
            <>
              <span className="user-name">{user.name}</span>
              <button type="button" className="btn btn-ghost btn-small" onClick={() => void logout()}>
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
        <span>Google sign-in · session on .filenkov.store</span>
      </footer>
    </div>
  )
}
