import { describe, it, expect } from 'vitest'
import { splitPathParts, sortVerbs, abbreviatePath, pathDepth } from '@/lib/pathUtils'

describe('splitPathParts', () => {
  it('handles static path', () => {
    const parts = splitPathParts('/api/v1/orders/')
    expect(parts.every((p) => !p.isDynamic)).toBe(true)
    expect(parts.map((p) => p.text).join('')).toBe('/api/v1/orders/')
  })

  it('identifies dynamic segments', () => {
    const parts = splitPathParts('/api/v1/orders/{pk}/')
    const dynamic = parts.filter((p) => p.isDynamic)
    expect(dynamic).toHaveLength(1)
    expect(dynamic[0].text).toBe('{pk}')
  })

  it('handles multiple dynamic segments', () => {
    const parts = splitPathParts('/api/v1/{org}/{repo}/commits/')
    const dynamic = parts.filter((p) => p.isDynamic)
    expect(dynamic).toHaveLength(2)
  })

  it('reconstructs original path', () => {
    const path = '/api/v1/widgets/{pk}/items/{item_id}/'
    const parts = splitPathParts(path)
    expect(parts.map((p) => p.text).join('')).toBe(path)
  })
})

describe('sortVerbs', () => {
  it('sorts verbs in canonical order', () => {
    const sorted = sortVerbs(['DELETE', 'GET', 'PATCH', 'POST'])
    expect(sorted).toEqual(['GET', 'POST', 'PATCH', 'DELETE'])
  })

  it('handles single verb', () => {
    expect(sortVerbs(['POST'])).toEqual(['POST'])
  })

  it('handles WS', () => {
    const sorted = sortVerbs(['WS', 'GET'])
    expect(sorted[0]).toBe('GET')
    expect(sorted[sorted.length - 1]).toBe('WS')
  })
})

describe('abbreviatePath', () => {
  it('returns short paths unchanged', () => {
    expect(abbreviatePath('/api/v1/users/')).toBe('/api/v1/users/')
  })

  it('abbreviates long paths', () => {
    const long = '/api/v1/organizations/departments/teams/members/'
    const abbrev = abbreviatePath(long, 30)
    expect(abbrev.startsWith('…/')).toBe(true)
    expect(abbrev.length).toBeLessThan(long.length)
  })
})

describe('pathDepth', () => {
  it('counts segments', () => {
    expect(pathDepth('/api/v1/orders/')).toBe(3)
    expect(pathDepth('/health/')).toBe(1)
    expect(pathDepth('/')).toBe(0)
  })
})
