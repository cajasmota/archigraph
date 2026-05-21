/**
 * useLayoutCache — persist / restore settled graph positions to localStorage.
 *
 * Key: `archigraph.layout.<group>.<nodesetHash>`
 * Value: base64-encoded Float32Array of [x0, y0, x1, y1, ...]
 *
 * Size limit: skip persist if encoded string > 2 MB (localStorage quota guard).
 * Node-set hash: deterministic 32-bit FNV-1a over sorted node IDs joined by ','.
 */

const MAX_BYTES = 2 * 1024 * 1024 // 2 MB guard

/** FNV-1a 32-bit hash of a string. Returns unsigned decimal string for use in keys. */
function fnv1a32(s: string): string {
  let h = 0x811c9dc5 >>> 0
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return String(h)
}

function layoutKey(group: string, nodeIds: string[]): string {
  const sorted = [...nodeIds].sort()
  const hash = fnv1a32(sorted.join(','))
  return `archigraph.layout.${group}.${hash}`
}

function float32ToBase64(arr: Float32Array): string {
  const bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength)
  let s = ''
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i])
  return btoa(s)
}

function base64ToFloat32(b64: string): Float32Array | null {
  try {
    const s = atob(b64)
    const bytes = new Uint8Array(s.length)
    for (let i = 0; i < s.length; i++) bytes[i] = s.charCodeAt(i)
    return new Float32Array(bytes.buffer)
  } catch {
    return null
  }
}

export interface LayoutCacheEntry {
  positions: Float32Array
}

export function saveLayout(group: string, nodeIds: string[], positions: Float32Array): void {
  try {
    const encoded = float32ToBase64(positions)
    if (encoded.length > MAX_BYTES) return // too large — skip silently
    const key = layoutKey(group, nodeIds)
    localStorage.setItem(key, encoded)
  } catch {
    // Ignore quota / private-mode errors
  }
}

export function loadLayout(group: string, nodeIds: string[]): LayoutCacheEntry | null {
  try {
    const key = layoutKey(group, nodeIds)
    const encoded = localStorage.getItem(key)
    if (!encoded) return null
    const positions = base64ToFloat32(encoded)
    if (!positions || positions.length !== nodeIds.length * 2) {
      localStorage.removeItem(key) // stale / corrupt
      return null
    }
    return { positions }
  } catch {
    return null
  }
}

export function clearLayout(group: string, nodeIds: string[]): void {
  try {
    localStorage.removeItem(layoutKey(group, nodeIds))
  } catch {
    // ignore
  }
}
