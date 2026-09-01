import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { Spine } from './Spine'

function renderSpine(items: { path: string; title: string }[]) {
  return render(
    <MemoryRouter>
      <Spine items={items} />
    </MemoryRouter>,
  )
}

describe('Spine', () => {
  it('renders the ancestor chain with the current node last', () => {
    renderSpine([
      { path: '', title: 'Start' },
      { path: 'experience', title: 'Experience' },
      { path: 'experience/sber', title: 'Sber' },
    ])

    const nav = screen.getByRole('navigation', { name: 'Resume path' })
    expect(nav).toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(3)
  })

  it('links the ancestors and marks the current node', () => {
    renderSpine([
      { path: '', title: 'Start' },
      { path: 'experience', title: 'Experience' },
      { path: 'experience/sber', title: 'Sber' },
    ])

    expect(screen.getByRole('link', { name: 'Start' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Experience' })).toHaveAttribute('href', '/experience')
    expect(screen.queryByRole('link', { name: 'Sber' })).not.toBeInTheDocument()
    expect(screen.getByText('Sber')).toHaveAttribute('aria-current', 'page')
  })

  it('renders nothing when the chain is empty', () => {
    const { container } = renderSpine([])
    expect(container).toBeEmptyDOMElement()
  })
})
