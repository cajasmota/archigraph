import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LodLevelIndicator } from '@/components/graph/LodLevelIndicator'

describe('LodLevelIndicator', () => {
  it('renders centroid label for zoom-out', () => {
    render(<LodLevelIndicator lodLevel="zoom-out" visibleCount={8} totalCount={5000} />)
    expect(screen.getByText('centroids')).toBeInTheDocument()
  })

  it('renders mid label', () => {
    render(<LodLevelIndicator lodLevel="mid" visibleCount={400} totalCount={5000} />)
    expect(screen.getByText('mid')).toBeInTheDocument()
  })

  it('renders blocked label', () => {
    render(<LodLevelIndicator lodLevel="blocked" visibleCount={0} totalCount={25000} />)
    expect(screen.getByText('blocked')).toBeInTheDocument()
  })

  it('shows visible count', () => {
    render(<LodLevelIndicator lodLevel="zoom-in" visibleCount={842} totalCount={842} />)
    expect(screen.getByText(/842/)).toBeInTheDocument()
  })

  it('has role=status for a11y', () => {
    render(<LodLevelIndicator lodLevel="mid" visibleCount={100} totalCount={5000} />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
