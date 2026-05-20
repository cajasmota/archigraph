import { describe, it, expect } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useCommunityColors, communityColor } from '@/hooks/graph/useCommunityColors'
import type { Community } from '@/types/api'

function makeCommunity(id: number): Community {
  return { id, repo: 'acme', size: 100, top_entities: [] }
}

describe('useCommunityColors', () => {
  it('returns a Map with one entry per community', () => {
    const communities = [makeCommunity(0), makeCommunity(1), makeCommunity(2)]
    const { result } = renderHook(() => useCommunityColors(communities))
    expect(result.current.size).toBe(3)
  })

  it('assigns stable colors — same id → same color across renders', () => {
    const communities = [makeCommunity(5), makeCommunity(11)]
    const { result, rerender } = renderHook(() => useCommunityColors(communities))
    const c5a = result.current.get(5)
    rerender()
    const c5b = result.current.get(5)
    expect(c5a).toBe(c5b)
  })

  it('returns hex strings', () => {
    const communities = [makeCommunity(0)]
    const { result } = renderHook(() => useCommunityColors(communities))
    const color = result.current.get(0)
    expect(color).toMatch(/^#[0-9a-f]{6}$/i)
  })
})

describe('communityColor', () => {
  it('returns a hex string', () => {
    expect(communityColor(0)).toMatch(/^#[0-9a-f]{6}$/i)
  })

  it('is stable for the same id', () => {
    expect(communityColor(7)).toBe(communityColor(7))
  })

  it('wraps past palette length', () => {
    // Community 0 and community 24 should both return valid colors
    const c0 = communityColor(0)
    const c24 = communityColor(24)
    expect(c24).toMatch(/^#[0-9a-f]{6}$/i)
    // They happen to equal each other (palette wraps at 24) — this is expected
    expect(c24).toBe(c0)
  })
})
