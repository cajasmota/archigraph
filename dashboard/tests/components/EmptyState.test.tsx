import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { EmptyState } from '@/components/shared/EmptyState'

// Mock lucide-react to avoid React version incompatibilities in jsdom
vi.mock('lucide-react', () => ({
  SearchX: (props: React.SVGProps<SVGSVGElement>) => <svg {...props} data-testid="icon" />,
  AlertTriangle: (props: React.SVGProps<SVGSVGElement>) => <svg {...props} />,
}))

describe('EmptyState', () => {
  it('renders title', () => {
    render(<EmptyState title="Nothing here" />)
    expect(screen.getByText('Nothing here')).toBeInTheDocument()
  })

  it('renders optional message', () => {
    render(<EmptyState title="No results" message="Try a different query" />)
    expect(screen.getByText('Try a different query')).toBeInTheDocument()
  })

  it('renders optional action', () => {
    render(<EmptyState title="Empty" action={<button>Retry</button>} />)
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('has role=status for a11y', () => {
    render(<EmptyState title="Nothing" />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
