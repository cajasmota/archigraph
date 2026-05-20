/**
 * Pure utility functions for Surface 4 (API Explorer) path handling.
 */

import type { HttpVerb } from '@/types/api'

// ────────────────────────────────────────────────────────────────────────────
// Verb ordering (consistent display order)
// ────────────────────────────────────────────────────────────────────────────

const VERB_ORDER: HttpVerb[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'ANY', 'WS']

export function sortVerbs(verbs: HttpVerb[]): HttpVerb[] {
  return [...verbs].sort((a, b) => VERB_ORDER.indexOf(a) - VERB_ORDER.indexOf(b))
}

// ────────────────────────────────────────────────────────────────────────────
// Path display helpers
// ────────────────────────────────────────────────────────────────────────────

/**
 * Highlights dynamic segments (e.g., {pk}) by splitting the path into parts
 * so callers can apply different styling to each part.
 */
export function splitPathParts(
  path: string,
): Array<{ text: string; isDynamic: boolean }> {
  const parts: Array<{ text: string; isDynamic: boolean }> = []
  const regex = /(\{[^}]+\})/g
  let lastIndex = 0
  let match: RegExpExecArray | null

  while ((match = regex.exec(path)) !== null) {
    if (match.index > lastIndex) {
      parts.push({ text: path.slice(lastIndex, match.index), isDynamic: false })
    }
    parts.push({ text: match[0], isDynamic: true })
    lastIndex = match.index + match[0].length
  }
  if (lastIndex < path.length) {
    parts.push({ text: path.slice(lastIndex), isDynamic: false })
  }
  return parts
}

/**
 * Compute the depth of a URL path (number of non-empty segments).
 * Used for indentation in the path tree.
 */
export function pathDepth(path: string): number {
  return path.split('/').filter(Boolean).length
}

/**
 * Truncate a long path for display, keeping the last 2 segments visible.
 */
export function abbreviatePath(path: string, maxLen = 60): string {
  if (path.length <= maxLen) return path
  const segments = path.split('/').filter(Boolean)
  if (segments.length <= 2) return path
  const tail = segments.slice(-2).join('/')
  return `…/${tail}${path.endsWith('/') ? '/' : ''}`
}

// ────────────────────────────────────────────────────────────────────────────
// Debounce
// ────────────────────────────────────────────────────────────────────────────

export function debounce<T extends (...args: unknown[]) => void>(fn: T, ms: number): T {
  let timer: ReturnType<typeof setTimeout>
  return ((...args: Parameters<T>) => {
    clearTimeout(timer)
    timer = setTimeout(() => fn(...args), ms)
  }) as T
}
