/**
 * Step-kind display metadata for the per-flow detail panel (#1150).
 *
 * STEP_KIND_COLORS maps each step_kind string to Tailwind utility classes for
 * background, text, and border — dark-mode-aware via dark: prefix.
 *
 * ENTRY_KIND_LABELS maps the entry_kind values returned by the API to
 * human-readable labels for the flow header badge.
 */

// ────────────────────────────────────────────────────────────────────────────
// Step-kind color map
// ────────────────────────────────────────────────────────────────────────────

export type StepKind =
  | 'http_fetch'
  | 'db_query'
  | 'db_write'
  | 'message_publish'
  | 'message_consume'
  | 'transform'
  | 'validation'
  | 'side_effect'
  | 'external_lib'
  | 'test_assert'
  | 'component_render'
  | 'render'
  | 'function_call'
  | 'unknown'

export interface StepKindSpec {
  bg: string
  text: string
  border: string
  /** Hex color used for React Flow node borders (canvas / SVG can't use Tailwind) */
  hex: string
  label: string
}

export const STEP_KIND_COLORS: Record<string, StepKindSpec> = {
  http_fetch: {
    bg: 'bg-blue-900/40',
    text: 'text-blue-300',
    border: 'border-blue-700',
    hex: '#60a5fa',
    label: 'HTTP Fetch',
  },
  db_query: {
    bg: 'bg-teal-900/40',
    text: 'text-teal-300',
    border: 'border-teal-700',
    hex: '#2dd4bf',
    label: 'DB Query',
  },
  db_write: {
    bg: 'bg-amber-900/40',
    text: 'text-amber-300',
    border: 'border-amber-700',
    hex: '#fbbf24',
    label: 'DB Write',
  },
  message_publish: {
    bg: 'bg-purple-900/40',
    text: 'text-purple-300',
    border: 'border-purple-700',
    hex: '#c084fc',
    label: 'Publish',
  },
  message_consume: {
    bg: 'bg-indigo-900/40',
    text: 'text-indigo-300',
    border: 'border-indigo-700',
    hex: '#818cf8',
    label: 'Consume',
  },
  transform: {
    bg: 'bg-slate-800/40',
    text: 'text-slate-300',
    border: 'border-slate-600',
    hex: '#94a3b8',
    label: 'Transform',
  },
  validation: {
    bg: 'bg-green-900/40',
    text: 'text-green-300',
    border: 'border-green-700',
    hex: '#4ade80',
    label: 'Validation',
  },
  side_effect: {
    bg: 'bg-orange-900/40',
    text: 'text-orange-300',
    border: 'border-orange-700',
    hex: '#fb923c',
    label: 'Side Effect',
  },
  external_lib: {
    bg: 'bg-slate-700/40',
    text: 'text-slate-400',
    border: 'border-slate-500',
    hex: '#64748b',
    label: 'External Lib',
  },
  test_assert: {
    bg: 'bg-lime-900/40',
    text: 'text-lime-300',
    border: 'border-lime-700',
    hex: '#a3e635',
    label: 'Assert',
  },
  component_render: {
    bg: 'bg-pink-900/40',
    text: 'text-pink-300',
    border: 'border-pink-700',
    hex: '#f472b6',
    label: 'Render',
  },
  render: {
    bg: 'bg-pink-900/40',
    text: 'text-pink-300',
    border: 'border-pink-700',
    hex: '#f472b6',
    label: 'Render',
  },
  function_call: {
    bg: 'bg-slate-800/40',
    text: 'text-slate-400',
    border: 'border-slate-600',
    hex: '#94a3b8',
    label: 'Function',
  },
  unknown: {
    bg: 'bg-slate-800/40',
    text: 'text-slate-400',
    border: 'border-slate-600',
    hex: '#94a3b8',
    label: 'Unknown',
  },
}

/** Returns the spec for a step_kind, falling back to function_call if unknown. */
export function stepKindSpec(kind: string | undefined): StepKindSpec {
  if (!kind) return STEP_KIND_COLORS.function_call
  return STEP_KIND_COLORS[kind] ?? STEP_KIND_COLORS.function_call
}

// ────────────────────────────────────────────────────────────────────────────
// Entry-kind labels
// ────────────────────────────────────────────────────────────────────────────

export const ENTRY_KIND_LABELS: Record<string, string> = {
  http: 'HTTP Handler',
  http_handler: 'HTTP Handler',
  kafka_consumer: 'Kafka Consumer',
  message_consumer: 'Message Consumer',
  scheduled: 'Scheduled Job',
  ws_handler: 'WebSocket Handler',
}

export function entryKindLabel(kind: string | undefined): string {
  if (!kind) return 'Entry'
  return ENTRY_KIND_LABELS[kind] ?? kind
}
