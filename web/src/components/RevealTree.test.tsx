import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { RevealTree } from './RevealTree'
import type { TreeItem } from '../resume/data'

const items: TreeItem[] = [
  {
    id: 'root',
    title: 'Root metric',
    summary: 'Saved money.',
    href: 'https://example.test',
    children: [
      {
        id: 'leaf',
        title: 'Leaf detail',
        summary: 'Used Kafka.',
      },
    ],
  },
]

describe('RevealTree', () => {
  it('keeps children closed until the branch is opened', async () => {
    const user = userEvent.setup()
    render(<RevealTree items={items} />)
    expect(screen.getByText('Saved money.')).toBeInTheDocument()
    expect(screen.queryByText('Used Kafka.')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open Root metric' })).toHaveAttribute('href', 'https://example.test')

    await user.click(screen.getByRole('button', { name: /Root metric/i }))
    expect(screen.getByText('Used Kafka.')).toBeInTheDocument()
    expect(screen.getByText('Deeper — more technical')).toBeInTheDocument()
  })
})
