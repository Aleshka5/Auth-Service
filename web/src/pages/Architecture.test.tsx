import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { Architecture } from './Architecture'

function renderArchitecture(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/architecture/:slug" element={<Architecture />} />
        <Route path="/" element={<div>home</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Architecture', () => {
  it('shows a coming-soon page for a known solution', () => {
    renderArchitecture('/architecture/auth-gateway')
    expect(screen.getByRole('heading', { name: 'Auth Gateway' })).toBeInTheDocument()
    expect(screen.getByText(/dedicated write-up/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open service' })).toHaveAttribute('href', '/auth')
    expect(screen.getByRole('link', { name: 'Back to the map' })).toHaveAttribute('href', '/#architecture')
  })

  it('sends unknown slugs back to the map', () => {
    renderArchitecture('/architecture/not-a-box')
    expect(screen.getByText('home')).toBeInTheDocument()
  })
})
