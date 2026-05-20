import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { GroupSwitcher } from '@/components/layout/GroupSwitcher'
import type { GroupMeta } from '@/types/api'

const mockGroups: GroupMeta[] = [
  {
    id: 'fixture-a',
    display_name: 'Fixture A',
    repos: [],
    entity_count: 6916,
    indexed_at: '2026-05-20T10:05:00Z',
  },
  {
    id: 'fixture-b',
    display_name: 'Fixture B',
    repos: [],
    entity_count: 4557,
    indexed_at: '2026-05-20T09:05:00Z',
  },
]

function renderWithRouter(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/:surface/:group" element={
          <GroupSwitcher groups={mockGroups} />
        } />
      </Routes>
    </MemoryRouter>,
  )
}

describe('GroupSwitcher', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders all groups', () => {
    renderWithRouter('/graph/fixture-a')
    expect(screen.getByText('Fixture A')).toBeDefined()
    expect(screen.getByText('Fixture B')).toBeDefined()
  })

  it('marks the active group as selected', () => {
    renderWithRouter('/graph/fixture-a')
    const activeOption = screen.getAllByRole('option').find(
      (o) => o.getAttribute('aria-selected') === 'true',
    )
    expect(activeOption?.textContent).toContain('Fixture A')
  })

  it('filters groups by query', () => {
    renderWithRouter('/graph/fixture-a')
    const input = screen.getByRole('searchbox', { name: 'Filter groups' })
    fireEvent.change(input, { target: { value: 'B' } })
    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(1)
    expect(options[0].textContent).toContain('Fixture B')
  })

  it('renders status dots for all groups', () => {
    renderWithRouter('/graph/fixture-a')
    // Each group option should have a status dot (role=none span with aria-label)
    const dots = screen.getAllByRole('option').flatMap(
      (o) => Array.from(o.querySelectorAll('[aria-label*="bug rate"]')),
    )
    expect(dots.length).toBeGreaterThanOrEqual(2)
  })

  it('calls onNavigate when a group is selected', () => {
    const onNavigate = vi.fn()
    render(
      <MemoryRouter initialEntries={['/graph/fixture-a']}>
        <Routes>
          <Route path="/:surface/:group" element={
            <GroupSwitcher groups={mockGroups} onNavigate={onNavigate} />
          } />
        </Routes>
      </MemoryRouter>,
    )
    // Click the outer button row for Fixture B (not the pin toggle)
    const fixtureB = screen.getAllByRole('option').find(
      (o) => o.textContent?.includes('Fixture B'),
    )!
    // Find the group name text span to click (avoids the pin toggle span)
    const nameSpan = fixtureB.querySelector('[class*="font-mono"]') as HTMLElement
    fireEvent.click(nameSpan ?? fixtureB)
    expect(onNavigate).toHaveBeenCalledOnce()
  })

  it('shows "No groups match" when filter has no results', () => {
    renderWithRouter('/graph/fixture-a')
    const input = screen.getByRole('searchbox', { name: 'Filter groups' })
    fireEvent.change(input, { target: { value: 'zzz' } })
    expect(screen.getByText('No groups match')).toBeDefined()
  })
})
