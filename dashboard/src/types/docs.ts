// Surface 5 — Docs Portal type contracts

// ────────────────────────────────────────────────────────────────────────────
// Doc tree (GET /api/docs/{group})
// ────────────────────────────────────────────────────────────────────────────

export interface DocTreeFile {
  type: 'file'
  path: string   // e.g. "acme-web/modules/auth/overview"
  label: string  // display name
  title?: string // h1 from doc
}

export interface DocTreeGroup {
  type: 'group'
  path: string
  label: string
  children: DocTreeNode[]
  defaultOpen?: boolean
}

export type DocTreeNode = DocTreeFile | DocTreeGroup

export interface DocTreeResponse {
  tree: DocTreeNode[]
  recentFiles: DocTreeFile[]
  repos: RepoMetadata[]
}

export interface RepoMetadata {
  slug: string
  display_name: string
}

// ────────────────────────────────────────────────────────────────────────────
// Doc content (GET /api/docs/{group}/{path})
// ────────────────────────────────────────────────────────────────────────────

export interface DocBreadcrumb {
  label: string
  path?: string  // undefined for the last (current) crumb
}

export interface DocNavLink {
  label: string
  path: string
}

export interface EntityCard {
  id: string
  label: string
  kind: string
  source_file: string
  start_line: number
  outbound_edges: Array<{ kind: string; target_label: string; target_id: string }>
}

export interface DocContentResponse {
  path: string
  title: string
  markdown: string
  breadcrumbs: DocBreadcrumb[]
  prev?: DocNavLink
  next?: DocNavLink
  /** Pre-resolved entity cards keyed by symbol label */
  hovercards?: Record<string, EntityCard>
}

// ────────────────────────────────────────────────────────────────────────────
// Search (GET /api/search/{group}?q=...)
// ────────────────────────────────────────────────────────────────────────────

export interface DocSearchResult {
  path: string
  title: string
  excerpt: string
  score: number
  repo?: string
}

export interface DocSearchResponse {
  results: DocSearchResult[]
  query: string
  total: number
}

// ────────────────────────────────────────────────────────────────────────────
// Parsed heading (for TOC + scroll-spy)
// ────────────────────────────────────────────────────────────────────────────

export interface TocHeading {
  id: string
  text: string
  depth: 2 | 3
  element?: Element  // populated at runtime
}
