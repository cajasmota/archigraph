import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useGraphSearch } from '@/hooks/graph/useGraphSearch'
import type { GraphNode } from '@/types/api'

function makeNode(id: string, label: string, pagerank = 0.1): GraphNode {
  return { id, label, kind: 'Component', repo: 'acme', pagerank }
}

const NODES: GraphNode[] = [
  makeNode('1', 'OrderService', 0.9),
  makeNode('2', 'OrderRepository', 0.5),
  makeNode('3', 'UserService', 0.4),
  makeNode('4', 'PaymentProcessor', 0.7),
  makeNode('5', 'createOrder', 0.2),
]

describe('useGraphSearch', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('returns empty results for empty query', () => {
    const { result } = renderHook(() => useGraphSearch('', NODES))
    expect(result.current.results).toHaveLength(0)
  })

  it('debounces and returns matching results', async () => {
    const { result, rerender } = renderHook(
      ({ q }) => useGraphSearch(q, NODES),
      { initialProps: { q: '' } },
    )
    rerender({ q: 'order' })
    // Before debounce fires — isSearching should be true
    expect(result.current.isSearching).toBe(true)
    // Fire debounce
    await act(async () => { vi.advanceTimersByTime(200) })
    // Now results should be populated
    expect(result.current.results.length).toBeGreaterThanOrEqual(2)
    const labels = result.current.results.map((n) => n.label)
    expect(labels).toContain('OrderService')
    expect(labels).toContain('OrderRepository')
    expect(result.current.isSearching).toBe(false)
  })

  it('ranks prefix matches above substring matches', async () => {
    const { result, rerender } = renderHook(
      ({ q }) => useGraphSearch(q, NODES),
      { initialProps: { q: '' } },
    )
    rerender({ q: 'order' })
    await act(async () => { vi.advanceTimersByTime(200) })
    // 'OrderService' (prefix) should appear before 'createOrder' (substring)
    const labels = result.current.results.map((n) => n.label)
    const orderSvcIdx = labels.indexOf('OrderService')
    const createOrderIdx = labels.indexOf('createOrder')
    if (createOrderIdx !== -1) {
      expect(orderSvcIdx).toBeLessThan(createOrderIdx)
    }
  })

  it('caps at 20 results', async () => {
    const manyNodes = Array.from({ length: 50 }, (_, i) =>
      makeNode(`n${i}`, `order_thing_${i}`),
    )
    const { result, rerender } = renderHook(
      ({ q }) => useGraphSearch(q, manyNodes),
      { initialProps: { q: '' } },
    )
    rerender({ q: 'order' })
    await act(async () => { vi.advanceTimersByTime(200) })
    expect(result.current.results.length).toBeLessThanOrEqual(20)
  })
})
