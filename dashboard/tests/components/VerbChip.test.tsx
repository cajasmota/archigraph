import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { VerbChip } from '@/components/shared/VerbChip'

describe('VerbChip', () => {
  it('renders the verb label', () => {
    render(<VerbChip verb="GET" />)
    expect(screen.getByText('GET')).toBeInTheDocument()
  })

  it('renders as a span (non-interactive) when no onClick', () => {
    render(<VerbChip verb="POST" />)
    const el = screen.getByText('POST')
    expect(el.tagName).toBe('SPAN')
  })

  it('renders as a button when onClick is provided', () => {
    render(<VerbChip verb="DELETE" onClick={() => undefined} />)
    const btn = screen.getByRole('button', { name: /filter by delete/i })
    expect(btn).toBeInTheDocument()
  })

  it('calls onClick when button is clicked', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<VerbChip verb="PUT" onClick={onClick} />)
    await user.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('renders with aria-pressed when active', () => {
    render(<VerbChip verb="PATCH" onClick={() => undefined} active />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'true')
  })

  it('renders all supported verbs without error', () => {
    const verbs = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'ANY', 'WS'] as const
    for (const verb of verbs) {
      const { unmount } = render(<VerbChip verb={verb} />)
      expect(screen.getByText(verb)).toBeInTheDocument()
      unmount()
    }
  })
})
