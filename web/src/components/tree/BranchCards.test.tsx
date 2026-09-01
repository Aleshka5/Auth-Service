import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { byPath } from '../../resume/registry'
import { stranger } from '../../test/fixtures'
import type { User } from '../../types'
import { BranchCards } from './BranchCards'

function renderCards(path: string, user: User | null = null, slug?: string) {
  return render(
    <MemoryRouter>
      <BranchCards node={byPath(path)!} slug={slug} user={user} />
    </MemoryRouter>,
  )
}

describe('BranchCards', () => {
  it('lists the root children in order and offers no way up', () => {
    renderCards('')
    const group = screen.getByRole('heading', { name: 'Where next?' }).parentElement!
    const titles = within(group)
      .getAllByRole('link')
      .map((link) => link.textContent)
    expect(titles[0]).toContain('Experience')
    expect(titles[1]).toContain('Achievements')
    expect(titles[2]).toContain('For interviewer')
    expect(titles[3]).toContain('About this project')
    expect(screen.queryByText(/Up to/)).not.toBeInTheDocument()
  })

  it('shows children, siblings and an up link on an inner node', () => {
    renderCards('experience/sber')
    expect(screen.getByRole('heading', { name: 'Deeper' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Or sideways' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Up to Experience/ })).toHaveAttribute('href', '/experience')
    expect(screen.getByRole('link', { name: 'AeroClub' })).toHaveAttribute('href', '/experience/aeroclub')
  })

  it('hides a signed-in child from a guest and shows it to a signed-in reader', () => {
    const { unmount } = renderCards('achievements/projects')
    expect(screen.queryByText('Hermes')).not.toBeInTheDocument()
    unmount()

    renderCards('achievements/projects', stranger)
    expect(screen.getByText('Hermes')).toBeInTheDocument()
  })

  it('gives a dynamic child only the way back up', () => {
    renderCards('about-project/architecture', null, 'auth-gateway')
    expect(screen.queryByRole('heading', { name: 'Deeper' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Up to Architecture/ })).toHaveAttribute(
      'href',
      '/about-project/architecture',
    )
  })
})
