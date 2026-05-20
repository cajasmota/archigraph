import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { EdgeBadge, edgeKindColor } from '@/components/graph/EdgeBadge'

describe('EdgeBadge', () => {
  it('renders kind label', () => {
    render(<EdgeBadge kind="CALLS" />)
    expect(screen.getByText('CALLS')).toBeInTheDocument()
  })

  it('shows cross-repo dot when crossRepo=true', () => {
    const { container } = render(<EdgeBadge kind="FETCHES" crossRepo />)
    // The dot span has aria-hidden
    const dot = container.querySelector('[aria-hidden]')
    expect(dot).toBeTruthy()
  })

  it('uses cross-repo title when crossRepo=true', () => {
    render(<EdgeBadge kind="IMPORTS" crossRepo />)
    expect(screen.getByTitle('IMPORTS (cross-repo)')).toBeInTheDocument()
  })
})

describe('edgeKindColor', () => {
  it('returns a hex string for known kinds', () => {
    const c = edgeKindColor('CALLS')
    expect(c).toMatch(/^#[0-9a-f]{6}$/i)
  })

  it('returns fallback for unknown kind', () => {
    // Cast to bypass type check
    const c = edgeKindColor('UNKNOWN' as Parameters<typeof edgeKindColor>[0])
    expect(c).toBe('#64748b')
  })
})
