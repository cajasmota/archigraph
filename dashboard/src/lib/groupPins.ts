/**
 * localStorage helpers for pinned/recent groups.
 * Max 3 pinned groups — oldest pin is evicted when the limit is exceeded.
 */

const STORAGE_KEY = 'archigraph:pinned-groups'
const MAX_PINS = 3

export function getPinnedGroups(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_PINS) : []
  } catch {
    return []
  }
}

export function pinGroup(groupId: string): string[] {
  const current = getPinnedGroups()
  if (current.includes(groupId)) return current
  const updated = [groupId, ...current].slice(0, MAX_PINS)
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(updated))
  } catch {
    // storage quota exceeded — degrade silently
  }
  return updated
}

export function unpinGroup(groupId: string): string[] {
  const current = getPinnedGroups()
  const updated = current.filter((id) => id !== groupId)
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(updated))
  } catch {
    // degrade silently
  }
  return updated
}

export function togglePin(groupId: string): string[] {
  const current = getPinnedGroups()
  return current.includes(groupId) ? unpinGroup(groupId) : pinGroup(groupId)
}
