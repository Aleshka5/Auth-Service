import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

function md(text: string, user = null as Parameters<typeof renderMarkdown>[1]) {
  return render(<MemoryRouter>{renderMarkdown(text, user)}</MemoryRouter>)
}

describe('renderMarkdown', () => {
  it('renders headings, emphasis and code spans', () => {
    const { container } = md('## Stack\nHe uses **CatBoost**, *DVC* and `pandas`.')

    expect(screen.getByRole('heading', { level: 4, name: 'Stack' })).toBeInTheDocument()
    expect(container.querySelector('strong')).toHaveTextContent('CatBoost')
    expect(container.querySelector('em')).toHaveTextContent('DVC')
    expect(container.querySelector('code')).toHaveTextContent('pandas')
  })

  it('renders bullet and ordered lists', () => {
    const { container } = md('- one\n- two\n\n1. first\n2. second')

    expect(container.querySelectorAll('ul > li')).toHaveLength(2)
    expect(container.querySelectorAll('ol > li')).toHaveLength(2)
    expect(container.querySelector('ol')).toHaveAttribute('start', '1')
  })

  it('nests a sub-list under its item', () => {
    const { container } = md('- outer\n  - inner')
    expect(container.querySelectorAll('ul')).toHaveLength(2)
    expect(container.querySelector('li ul li')).toHaveTextContent('inner')
  })

  it('renders a fenced code block verbatim', () => {
    const { container } = md('Try:\n```go\nfunc main() {}\n```')
    const pre = container.querySelector('pre code')
    expect(pre).toHaveTextContent('func main() {}')
    // Markdown inside a fence is not markdown.
    expect(container.querySelector('pre strong')).toBeNull()
  })

  it('keeps an unclosed fence readable while the answer is still streaming', () => {
    const { container } = md('```\nhalf a li')
    expect(container.querySelector('pre code')).toHaveTextContent('half a li')
  })

  it('renders a blockquote and a rule', () => {
    const { container } = md('> quoted line\n\n---')
    expect(container.querySelector('blockquote')).toHaveTextContent('quoted line')
    expect(container.querySelector('hr')).toBeInTheDocument()
  })

  it('renders a table with its header row', () => {
    const { container } = md('| Metric | Value |\n| --- | --- |\n| Top-1 | 65% |')

    expect(screen.getByRole('columnheader', { name: 'Metric' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '65%' })).toBeInTheDocument()
    expect(container.querySelectorAll('tbody tr')).toHaveLength(1)
  })

  it('leaves pipes alone when there is no divider row', () => {
    const { container } = md('a | b | c')
    expect(container.querySelector('table')).toBeNull()
    expect(container.textContent).toBe('a | b | c')
  })

  it('still links a bare page path in prose', () => {
    md('The numbers are on /experience/sber.')
    expect(screen.getByRole('link', { name: '/experience/sber' })).toHaveAttribute(
      'href',
      '/experience/sber',
    )
  })

  it('links a markdown link to a real page and drops an invented one', () => {
    md('See [Sber](/experience/sber) and [nothing](/experience/google).')
    expect(screen.getByRole('link', { name: 'Sber' })).toHaveAttribute('href', '/experience/sber')
    expect(screen.queryByRole('link', { name: 'nothing' })).toBeNull()
  })

  it('opens an external link in a new tab', () => {
    md('[GitHub](https://github.com/Aleshka5)')
    const link = screen.getByRole('link', { name: 'GitHub' })
    expect(link).toHaveAttribute('href', 'https://github.com/Aleshka5')
    expect(link).toHaveAttribute('rel', 'noreferrer noopener')
  })

  it('refuses a script-scheme link and keeps its label as text', () => {
    const { container } = md('[click](javascript:alert)')
    expect(container.querySelector('a')).toBeNull()
    expect(container.textContent).toBe('click')
  })

  it('never turns html in the answer into markup', () => {
    const { container } = md('<img src=x onerror="alert(1)"> and <b>bold</b>')
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container.textContent).toContain('<b>bold</b>')
  })
})
