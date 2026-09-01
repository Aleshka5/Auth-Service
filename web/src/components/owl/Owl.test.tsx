import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../../auth'
import { jsonResponse, stranger } from '../../test/fixtures'
import { Owl } from './Owl'

let scrollTo: ReturnType<typeof vi.fn>

beforeEach(() => {
  scrollTo = vi.fn()
  vi.stubGlobal('scrollTo', scrollTo)
  vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  delete document.documentElement.dataset.owl
  document.getElementById('node-article')?.remove()
})

function renderOwl() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <AuthProvider>
        <Owl />
      </AuthProvider>
    </MemoryRouter>,
  )
}

/** Stand-in for the article NodeView renders, whose h1 already has tabIndex -1. */
function mountArticle() {
  const article = document.createElement('article')
  article.id = 'node-article'
  const h1 = document.createElement('h1')
  h1.tabIndex = -1
  h1.textContent = 'Node title'
  article.append(h1)
  document.body.append(article)
  return h1
}

describe('Owl', () => {
  it('is a button labelled "Back to top", with a hero slot holding the flow', async () => {
    renderOwl()
    const button = await screen.findByRole('button', { name: 'Back to top' })
    expect(button).toHaveClass('owl')
    expect(button.tagName).toBe('BUTTON')
    expect(button.getAttribute('type')).toBe('button')
    expect(document.querySelector('.owl-slot')).not.toBeNull()
    expect(button.querySelector('.owl-ring')).not.toBeNull()
    expect(button.querySelector('.owl-art')).not.toBeNull()
  })

  it('scrolls to the top and focuses the article heading while docked', async () => {
    const h1 = mountArticle()
    renderOwl()
    const button = await screen.findByRole('button', { name: 'Back to top' })
    document.documentElement.dataset.owl = 'docked'

    fireEvent.click(button)

    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    expect(document.activeElement).toBe(h1)
  })

  it('does nothing while it is still the hero', async () => {
    mountArticle()
    renderOwl()
    const button = await screen.findByRole('button', { name: 'Back to top' })
    document.documentElement.dataset.owl = 'hero'

    fireEvent.click(button)

    expect(scrollTo).not.toHaveBeenCalled()
    expect(document.activeElement).not.toBe(document.querySelector('#node-article h1'))
  })

  it('injects the mood clip for the current auth state into both owls', async () => {
    renderOwl()
    const button = await screen.findByRole('button', { name: 'Back to top' })
    const hero = document.querySelector('.owl-hero')
    expect(hero).not.toBeNull()

    // The hero owl carries the mascot attributes; the docked one is the same art.
    await vi.waitFor(() => {
      expect(hero?.getAttribute('data-mascot')).toBe('user')
    })
    expect(hero?.getAttribute('data-mascot-motion')).toBe('angry-to-calm')
    expect(hero?.querySelector('svg')).not.toBeNull()
    expect(button.querySelector('.owl-art svg')).not.toBeNull()
  })
})
