import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { admin, stranger } from '../../test/fixtures'
import { withNodeLinks } from './nodeRefs'

function renderText(text: string, user = null as Parameters<typeof withNodeLinks>[1]) {
  return render(<MemoryRouter>{withNodeLinks(text, user)}</MemoryRouter>)
}

describe('withNodeLinks', () => {
  it('links a real node path and keeps the sentence intact', () => {
    const { container } = renderText('The numbers are on /experience/sber, have a look.')

    const link = screen.getByRole('link', { name: '/experience/sber' })
    expect(link).toHaveAttribute('href', '/experience/sber')
    expect(link).toHaveAttribute('title', 'Sber')
    // Punctuation stays outside the link.
    expect(container.textContent).toBe('The numbers are on /experience/sber, have a look.')
  })

  it('links a dynamic child of the architecture node', () => {
    renderText('See /about-project/architecture/auth-gateway for the box.')
    expect(screen.getByRole('link', { name: '/about-project/architecture/auth-gateway' })).toHaveAttribute(
      'href',
      '/about-project/architecture/auth-gateway',
    )
  })

  it('leaves an invented path as plain text', () => {
    const { container } = renderText('Try /experience/google or /salary.')
    expect(screen.queryAllByRole('link')).toHaveLength(0)
    expect(container.textContent).toBe('Try /experience/google or /salary.')
  })

  it('never links a node the reader may not see', () => {
    const hidden = '/achievements/projects/hermes'
    renderText(`Look at ${hidden}.`)
    expect(screen.queryByRole('link')).toBeNull()

    renderText(`Look at ${hidden}.`, stranger)
    expect(screen.getByRole('link', { name: hidden })).toBeInTheDocument()
  })

  it('links every path in a longer answer', () => {
    renderText('Both /experience/sber and /experience/aeroclub are worth reading.')
    expect(screen.getAllByRole('link')).toHaveLength(2)
  })

  it('ignores a bare slash and a partial path mid-stream', () => {
    const { container } = renderText('and / then /experi')
    expect(screen.queryAllByRole('link')).toHaveLength(0)
    expect(container.textContent).toBe('and / then /experi')
  })

  it('is unaffected by the admin flag on an always-visible node', () => {
    renderText('/for-interviewer/logistics', admin)
    expect(screen.getByRole('link')).toHaveAttribute('href', '/for-interviewer/logistics')
  })
})
