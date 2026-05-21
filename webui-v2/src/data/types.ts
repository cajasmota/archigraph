/* ============================================================
   data/types.ts — the typed domain model.

   These shapes mirror the archigraph daemon's responses and the
   per-screen "Data model" sections in the design handoff docs.
   Screen tickets extend this file as they wire real endpoints.
   ============================================================ */

export type EdgeKind =
  | "CALLS"
  | "REFERENCES"
  | "RENDERS"
  | "DEPENDS_ON"
  | "EXTENDS"
  | "CONTAINS"
  | "IMPORTS";

export interface Repo {
  id: string;
  name: string;
  /** Primary language label (drives the Stack badge). */
  language: string;
}

export interface Community {
  id: string;
  label: string;
  /** 1-based index into the pastel categorical scale. */
  colorIndex: number;
  size: number;
}

export interface Entity {
  id: string;
  /** Qualified name — rendered in mono. */
  qualifiedName: string;
  kind: string;
  repoId: string;
  communityId: string | null;
  inbound: number;
  outbound: number;
}

/** Derived health state for a group (computed server-side in v2_groups.go). */
export type GroupHealth = "healthy" | "warning" | "unindexed";

export interface Group {
  /** Slug — also the route param. */
  id: string;
  name: string;
  /** Top-level repo slugs. */
  repos: string[];
  entityCount: number;
  /**
   * Confidence the graph matches the codebase, 0–1. `null` when the group
   * has never been indexed. (Replaces the legacy "bug-rate".)
   */
  fidelity: number | null;
  /** ms epoch of the most-recent index across repos; `null` when never indexed. */
  indexedAt: number | null;
  health: GroupHealth;
}
