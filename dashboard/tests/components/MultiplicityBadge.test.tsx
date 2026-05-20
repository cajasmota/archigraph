import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MultiplicityBadge } from '@/components/paths/MultiplicityBadge'

describe('MultiplicityBadge', () => {
  it('renders nothing when count is 1', () => {
    const { container } = render(<MultiplicityBadge count={1} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when count is 0', () => {
    const { container } = render(<MultiplicityBadge count={0} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders ×N for count > 1', () => {
    render(<MultiplicityBadge count={5} />)
    expect(screen.getByText('×5')).toBeInTheDocument()
  })

  it('has accessible label', () => {
    render(<MultiplicityBadge count={3} />)
    expect(screen.getByLabelText('3 endpoints')).toBeInTheDocument()
  })
})
