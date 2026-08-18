import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Layout } from './Layout'
import { admin, jsonResponse } from '../test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderLayout() {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<div>outlet</div>} />
          </Route>
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('Layout', () => {
  it('shows Sign in for anonymous users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderLayout()
    expect(await screen.findByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/auth')
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('shows the name and Admin link for admins', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(admin)))
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Admin' })).toHaveAttribute('href', '/admin')
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })
})
