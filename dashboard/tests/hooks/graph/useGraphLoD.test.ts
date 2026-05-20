import { describe, it, expect } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useGraphLoD } from '@/hooks/graph/useGraphLoD'
import type { GraphNode, GraphEdge, Community } from '@/types/api'

function makeNode(overrides: Partial<GraphNode> = {}): GraphNode {
  return {
    id: 'n1',
    label: 'TestNode',
    kind: 'Component',
    repo: 'acme',
    community_id: 0,
    pagerank: 0.1,
    ...overrides,
  }
}

function makeEdge(src: string, tgt: string): GraphEdge {
  return {
    id: `${src}::${tgt}::CALLS`,
    source: src,
    target: tgt,
    kind: 'CALLS',
  }
}

const emptyCommunities: Community[] = []

describe('useGraphLoD', () => {
  it('returns zoom-out when total >= 5000', () => {
    const nodes = Array.from({ length: 5000 }, (_, i) =>
      makeNode({ id: `n${i}`, is_centroid: i < 8 }),
    )
    const { result } = renderHook(() =>
      useGraphLoD(nodes, [], emptyCommunities, 1.0, null, null),
    )
    expect(result.current.lodLevel).toBe('zoom-out')
  })

  it('returns zoom-out when zoomLevel < 0.5', () => {
    const nodes = [makeNode({ id: 'n1', is_centroid: true })]
    const { result } = renderHook(() =>
      useGraphLoD(nodes, [], emptyCommunities, 0.3, null, null),
    )
    expect(result.current.lodLevel).toBe('zoom-out')
  })

  it('returns mid when 1000 <= total < 5000', () => {
    const nodes = Array.from({ length: 1500 }, (_, i) => makeNode({ id: `n${i}` }))
    const { result } = renderHook(() =>
      useGraphLoD(nodes, [], emptyCommunities, 1.0, null, null),
    )
    expect(result.current.lodLevel).toBe('mid')
  })

  it('returns zoom-in when total < 1000 and zoom >= 2.0', () => {
    const nodes = Array.from({ length: 500 }, (_, i) => makeNode({ id: `n${i}` }))
    const { result } = renderHook(() =>
      useGraphLoD(nodes, [], emptyCommunities, 2.5, null, null),
    )
    expect(result.current.lodLevel).toBe('zoom-in')
  })

  it('returns blocked when total > 20000', () => {
    const nodes = Array.from({ length: 20001 }, (_, i) => makeNode({ id: `n${i}` }))
    const { result } = renderHook(() =>
      useGraphLoD(nodes, [], emptyCommunities, 1.0, null, null),
    )
    expect(result.current.lodLevel).toBe('blocked')
    expect(result.current.visibleNodeIds.size).toBe(0)
    expect(result.current.visibleEdgeIds.size).toBe(0)
  })

  it('always includes selected node and 1-hop neighbors', () => {
    // 5000 nodes → zoom-out → normally only centroids visible
    const centroid = makeNode({ id: 'centroid-0', is_centroid: true, community_id: 0 })
    const selected = makeNode({ id: 'selected', community_id: 1 })
    const neighbor = makeNode({ id: 'neighbor', community_id: 2 })
    const unrelated = makeNode({ id: 'unrelated', community_id: 3 })
    // Pad to trigger zoom-out
    const padding = Array.from({ length: 4997 }, (_, i) => makeNode({ id: `pad${i}` }))
    const nodes = [centroid, selected, neighbor, unrelated, ...padding]
    const edges = [makeEdge('selected', 'neighbor')]
    const { result } = renderHook(() =>
      useGraphLoD(nodes, edges, emptyCommunities, 1.0, null, 'selected'),
    )
    expect(result.current.lodLevel).toBe('zoom-out')
    expect(result.current.visibleNodeIds.has('selected')).toBe(true)
    expect(result.current.visibleNodeIds.has('neighbor')).toBe(true)
    expect(result.current.visibleNodeIds.has('unrelated')).toBe(false)
  })

  it('culls edges when one endpoint is not visible', () => {
    const n1 = makeNode({ id: 'n1' })
    const n2 = makeNode({ id: 'n2' })  // not visible in zoom-out
    // 5000 total → zoom-out, no centroids → only selected + neighbors
    const padding = Array.from({ length: 4998 }, (_, i) => makeNode({ id: `pad${i}` }))
    const nodes = [n1, n2, ...padding]
    const edges = [makeEdge('n1', 'n2')]
    const { result } = renderHook(() =>
      useGraphLoD(nodes, edges, emptyCommunities, 1.0, null, null),
    )
    // n2 not visible → edge culled
    expect(result.current.visibleEdgeIds.has('n1::n2::CALLS')).toBe(false)
  })
})
