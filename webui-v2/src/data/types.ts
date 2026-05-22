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

/* ------------------------------------------------------------------ *
 * Graph screen — wire + domain shapes.
 *
 * The wire shapes (snake_case) mirror v2_graph.go's JSON exactly. The
 * data hook normalizes them into the camelCase domain shapes the screen +
 * cosmos canvas consume.
 * ------------------------------------------------------------------ */

/** Raw node as emitted by GET /api/v2/graph/{group}. */
export interface GraphNodeWire {
  id: string;
  label: string;
  kind: string;
  repo: string;
  degree: number;
  pagerank: number;
  community_id?: number;
  source_file?: string;
}

export interface GraphEdgeWire {
  source: string;
  target: string;
  kind: string;
}

export interface GraphCommunityWire {
  id: number;
  label: string;
  repo: string;
  size: number;
  color_index: number;
}

export interface GraphRepoWire {
  id: string;
  language: string;
  color_index: number;
}

export interface GraphPayloadWire {
  nodes: GraphNodeWire[];
  edges: GraphEdgeWire[];
  communities: GraphCommunityWire[];
  repos: GraphRepoWire[];
  total_node_count: number;
}

/** Normalized node consumed by the canvas + inspector. */
export interface GraphNode {
  id: string;
  label: string;
  kind: string;
  repo: string;
  degree: number;
  pageRank: number;
  communityId: number | null;
  sourceFile: string;
}

export interface GraphEdge {
  source: string;
  target: string;
  kind: string;
}

export interface GraphCommunity {
  id: number;
  label: string;
  repo: string;
  size: number;
  colorIndex: number;
}

export interface GraphRepo {
  id: string;
  language: string;
  colorIndex: number;
}

export interface GraphPayload {
  nodes: GraphNode[];
  edges: GraphEdge[];
  communities: GraphCommunity[];
  repos: GraphRepo[];
  totalNodeCount: number;
}

/** Tier-3 entity detail — GET /api/graph/{group}/entity/{id} (v1, reused). */
export interface EntityEdgeWire {
  from_id: string;
  to_id: string;
  kind: string;
  cross_repo?: boolean;
}
export interface EntityNeighborWire {
  id: string;
  label: string;
  kind: string;
  source_file: string;
  start_line: number;
  repo: string;
}
export interface EntityDetailWire {
  entity: {
    id: string;
    name: string;
    kind: string;
    source_file: string;
    start_line: number;
    repo?: string;
    pagerank?: number;
  };
  inbound_edges: EntityEdgeWire[];
  outbound_edges: EntityEdgeWire[];
  neighbors: EntityNeighborWire[];
  in_degree: number;
  out_degree: number;
  community_name?: string;
  betweenness?: number;
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

// ── Docs screen ─────────────────────────────────────────────────────────────

export type DocsEntityKind =
  | "function"
  | "component"
  | "hook"
  | "class"
  | "method"
  | "http_endpoint"
  | "module"
  | "folder"
  | "repo";

export interface DocsTreeNode {
  type: DocsEntityKind;
  name: string;
  id?: string;           // leaf only
  children?: DocsTreeNode[];
}

export interface DocsParam {
  name: string;
  type: string;
  desc: string;
}

export interface DocsEntityDetail {
  name: string;
  type: DocsEntityKind;
  repo: string;
  file: string;
  line: number;
  signature: string;
  description: string;
  aiGenerated: boolean;
  params: DocsParam[];
  returns: { type: string; desc?: string } | null;
  inbound: number;
  outbound: number;
  callers: string[];
  callees: string[];
  responseShapes?: { status: number; shape: string }[];
  stub?: boolean;
}

// ----------------------------------------------------------------
// Settings screen types (mirrors v2_group_settings.go wire shapes)
// ----------------------------------------------------------------

export interface SettingsFeatures {
  watchers: boolean;
  gitHooks: boolean;
}

export interface MonorepoPkg {
  path: string;
  stack: string;
  indexed: boolean;
  files: number;
}

export interface MonorepoInfo {
  detector: string;
  packages: MonorepoPkg[];
}

export interface SettingsRepo {
  slug: string;
  path: string;
  stack: string;
  files: number;
  entities: number;
  indexedAt: number | null;
  monorepo: MonorepoInfo | null;
}

export interface SettingsGroup {
  id: string;
  name: string;
  entities: number;
  fidelity: number;
  indexedAt: number | null;
  health: GroupHealth;
  features: SettingsFeatures;
  docsPath: string;
  repos: SettingsRepo[];
}

export interface DoctorCheck {
  id: string;
  label: string;
  status: "ok" | "warning" | "info" | "error";
  detail: string;
}

// ── Topology screen ──────────────────────────────────────────────────────────

/** Canonical broker identifiers used for color/shape mapping. */
export type BrokerCanonical =
  | "kafka"
  | "rabbitmq"
  | "sqs"
  | "pubsub"
  | "nats"
  | "websocket"
  | "sse"
  | "graphql_subscription"
  | "redis_pubsub"
  | "redis"
  | "redis-stream"
  | "celery"
  | "task-queue"
  | "serverless"
  | "unknown"
  | (string & Record<never, never>); // allow extension strings

/** Lifecycle state of a channel (producer/consumer presence). */
export type ChannelLifecycle =
  | "active"
  | "orphan_publisher"
  | "orphan_subscriber"
  | "orphan";

/**
 * Wire shape for a single channel (topic/queue/sse/ws/graphql-sub/serverless).
 * Mirrors the JSON produced by GET /api/topology/:group (non-v2).
 * Critical: no `last_message_seen`, no `usage_history` — those are always null/[].
 */
export interface TopologyChannel {
  id: string;
  label: string;
  broker: string;
  broker_canonical: BrokerCanonical;
  framework?: string;
  owning_service: string;
  producers: string[];   // entity ids
  consumers: string[];
  scheduled?: boolean;
  schedule?: string;
  repo: string;
  channel_type?: "websocket" | "sse" | "redis_pubsub" | "graphql_subscription";
  // optional enrichment fields (present only after /generate-docs)
  docs_summary?: string;
  docgen_status?: "enriched" | "stale" | "pending";
  // cross-repo flag (derived client-side from broker_groups)
  cross_repo?: boolean;
}

/** Serverless function entry in the topology payload. */
export interface TopologyFunction {
  id: string;
  label: string;
  repo: string;
  provider?: string;
  invokers: string[];
  handlers: string[];
}

/** Transform edge (channel → channel). */
export interface TopologyTransform {
  from_id: string;
  to_id: string;
  repo: string;
}

/** Per-service aggregated stats inside a broker group. */
export interface BrokerServiceStat {
  name: string;
  topic_count: number;
}

/** Health breakdown per broker. */
export interface BrokerHealthSummary {
  active: number;
  orphan_publisher: number;
  orphan_subscriber: number;
  orphan: number;
}

/** One element of `broker_groups` in the topology payload. */
export interface TopologyBrokerGroup {
  broker: BrokerCanonical;
  count: number;
  services: BrokerServiceStat[];
  orphan_publishers: number;
  orphan_subscribers: number;
  cross_repo_topic_count: number;
  health_summary: BrokerHealthSummary;
  last_index_timestamp?: string; // ISO-8601
}

/**
 * Full wire response from GET /api/topology/:group.
 * All array fields are guaranteed non-null by the daemon.
 */
export interface TopologyResponse {
  topics: TopologyChannel[];
  queues: TopologyChannel[];
  channels: TopologyChannel[];
  nats_subjects: TopologyChannel[];
  graphql_subscriptions: TopologyChannel[];
  functions: TopologyFunction[];
  transforms: TopologyTransform[];
  broker_groups: TopologyBrokerGroup[];
}

/** Detailed channel view — GET /api/topology/:group/topic/:topicId. */
export interface TopologyChannelDetail extends TopologyChannel {
  source_file: string;
  start_line: number;
  protocol: string;
  message_schema?: string;
  tests: string[];
  related_topics: { id: string; label: string; broker_canonical: BrokerCanonical }[];
  flow_count: number;
  cross_repo: boolean;
  lifecycle_state: ChannelLifecycle;
  enrichment_health?: {
    has_summary: boolean;
    has_schema: boolean;
    has_volume_estimate: boolean;
    has_typical_payload_size: boolean;
    has_expected_consumers: boolean;
    has_gaps: boolean;
    filled_field_count: number;
    total_field_count: number;
  };
}

/** Orphan publisher entry — GET /api/topology/:group/orphan-publishers. */
export interface OrphanPublisherEntry {
  id: string;
  label: string;
  broker_canonical: BrokerCanonical;
  repo: string;
  producers: string[];
  reason?: string;
}

/** Orphan subscriber entry — GET /api/topology/:group/orphan-subscribers. */
export interface OrphanSubscriberEntry {
  id: string;
  label: string;
  broker_canonical: BrokerCanonical;
  repo: string;
  consumers: string[];
  reason?: string;
}
