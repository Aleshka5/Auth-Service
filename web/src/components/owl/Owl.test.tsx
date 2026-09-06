import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../../auth'
import { jsonResponse, stranger } from '../../test/fixtures'
import { GREETING } from './greeting'
import { Owl } from './Owl'

let scrollTo: ReturnType<typeof vi.fn>

beforeEach(() => {
  scrollTo = vi.fn()
  vi.stubGlobal('scrollTo', scrollTo)
  vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
  sessionStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  delete document.documentElement.dataset.owl
  document.getElementById('node-article')?.remove()
})

function renderOwl(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
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

  it('greets the reader beside the hero owl on every arrival', async () => {
    const { unmount } = renderOwl()

    // The bubble is announced whole, and types itself out for the eye.
    const bubble = await screen.findByRole('status')
    expect(bubble).toHaveTextContent(GREETING)
    // It arrives a letter at a time, so give the typing more than the default second.
    await waitFor(
      () => {
        expect(bubble.querySelector('[aria-hidden="true"]')?.textContent).toBe(GREETING)
      },
      { timeout: 5000 },
    )

    // No once-per-session gate: entering the site again greets again.
    unmount()
    renderOwl()
    expect(await screen.findByRole('status')).toHaveTextContent(GREETING)
  })

  it('never opens the chat on its own — the greeting only invites the click', async () => {
    renderOwl()

    const hero = await screen.findByRole('button', { name: 'Chat with the AI owl' })
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('Ask me something')).toBeNull()
    expect(hero.getAttribute('aria-expanded')).toBe('false')
  })

  it('leaves a node page alone: no greeting, no panel', async () => {
    renderOwl('/experience/sber')
    await screen.findByRole('button', { name: 'Chat with the AI owl' })
    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.queryByPlaceholderText('Ask me something')).toBeNull()
  })

  it.each(['/', '/experience/sber'])('opens on click at %s, and focuses the field', async (path) => {
    renderOwl(path)
    const hero = await screen.findByRole('button', { name: 'Chat with the AI owl' })

    await userEvent.click(hero)

    const input = await screen.findByPlaceholderText('Ask me something')
    expect(hero.getAttribute('aria-expanded')).toBe('true')
    // Opened deliberately, so the caret belongs in the field.
    await waitFor(() => expect(document.activeElement).toBe(input))
  })

  it('drops the greeting once the reader takes it up', async () => {
    renderOwl()
    await screen.findByRole('status')

    await userEvent.click(screen.getByRole('button', { name: 'Chat with the AI owl' }))

    expect(await screen.findByPlaceholderText('Ask me something')).toBeInTheDocument()
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('dismissing the greeting does not open anything', async () => {
    renderOwl()
    await screen.findByRole('status')

    await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }))

    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.queryByPlaceholderText('Ask me something')).toBeNull()
  })

  it('closes again and stays closed', async () => {
    renderOwl()
    await userEvent.click(screen.getByRole('button', { name: 'Chat with the AI owl' }))
    await screen.findByPlaceholderText('Ask me something')

    await userEvent.click(screen.getByRole('button', { name: 'Close chat' }))

    expect(screen.queryByPlaceholderText('Ask me something')).toBeNull()
    expect(screen.getByRole('button', { name: 'Chat with the AI owl' }).getAttribute('aria-expanded')).toBe('false')
  })
})
