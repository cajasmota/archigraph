/**
 * FlowDag — per-flow React Flow DAG visualization (#1150).
 *
 * Renders a directed acyclic graph of process steps using @xyflow/react v12.
 * Auto-layout via dagre. Custom node per step_kind. Mini-map + background grid.
 *
 * This module is lazy-loaded at the route boundary — do NOT import it directly
 * from FlowDetailPanel.tsx; use FlowDagLazy.tsx instead.
 *
 * Jarvis hook: useGraphHighlight reserve for MCP-driven step highlight (#1157).
 */

import { useCallback, useEffect, useMemo } from 'react'
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  addEdge,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Node,
  type Edge,
  type OnConnect,
  type NodeTypes,
  type FitViewOptions,
  type NodeReplaceChange,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from 'dagre'
import {
  Globe,
  Database,
  ArrowUpFromLine,
  Send,
  Download,
  Wrench,
  ShieldCheck,
  AlertTriangle,
  Package,
  CheckCircle,
  LayoutTemplate,
  Circle,
} from 'lucide-react'
import { stepKindSpec } from '@/lib/flowKinds'
import type { ProcessStep } from '@/types/api'

// ────────────────────────────────────────────────────────────────────────────
// Constants
// ────────────────────────────────────────────────────────────────────────────

const NODE_WIDTH = 180
const NODE_HEIGHT = 56
const DAGRE_RANK_SEP = 80
const DAGRE_NODE_SEP = 40

const FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: 0.2,
  maxZoom: 1.5,
}

// ────────────────────────────────────────────────────────────────────────────
// Step-kind icon
// ────────────────────────────────────────────────────────────────────────────

function StepKindIcon({ kind, className }: { kind: string | undefined; className?: string }) {
  const cn = className ?? 'w-4 h-4'
  switch (kind) {
    case 'http_fetch':       return <Globe className={cn} />
    case 'db_query':         return <Database className={cn} />
    case 'db_write':         return <ArrowUpFromLine className={cn} />
    case 'message_publish':  return <Send className={cn} />
    case 'message_consume':  return <Download className={cn} />
    case 'transform':        return <Wrench className={cn} />
    case 'validation':       return <ShieldCheck className={cn} />
    case 'side_effect':      return <AlertTriangle className={cn} />
    case 'external_lib':     return <Package className={cn} />
    case 'test_assert':      return <CheckCircle className={cn} />
    case 'component_render':
    case 'render':           return <LayoutTemplate className={cn} />
    default:                 return <Circle className={cn} />
  }
}

// ────────────────────────────────────────────────────────────────────────────
// Custom step node
// ────────────────────────────────────────────────────────────────────────────

interface StepNodeData {
  label: string
  step_kind?: string
  repo: string
  primaryRepo: string
  step_index: number
  source_file: string
  start_line: number
  entity_id: string
  [key: string]: unknown
}

function StepNode({ data, selected }: { data: StepNodeData; selected?: boolean }) {
  const spec = stepKindSpec(data.step_kind)
  const isCrossRepo = data.repo !== data.primaryRepo

  return (
    <div
      className={[
        'flex items-center gap-2 px-3 rounded-lg border transition-all',
        'bg-slate-900 dark:bg-slate-950',
        selected
          ? 'border-sky-500 shadow-lg shadow-sky-500/20'
          : 'border-slate-700 hover:border-slate-500',
      ].join(' ')}
      style={{
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
        borderLeftWidth: 3,
        borderLeftColor: spec.hex,
      }}
    >
      {/* Step kind icon */}
      <span className={spec.text} aria-hidden>
        <StepKindIcon kind={data.step_kind} className="w-3.5 h-3.5 flex-shrink-0" />
      </span>

      {/* Label */}
      <div className="flex-1 min-w-0">
        <p
          className="font-mono text-[11px] text-slate-200 truncate leading-tight"
          title={data.label}
        >
          {data.label}
        </p>
        {isCrossRepo && (
          <p className="text-[9px] text-violet-400 font-mono truncate mt-0.5">{data.repo}</p>
        )}
      </div>

      {/* Step index */}
      <span className="text-[9px] text-slate-600 font-mono flex-shrink-0">
        {data.step_index + 1}
      </span>
    </div>
  )
}

// ────────────────────────────────────────────────────────────────────────────
// Node types registry (memoized outside render to keep React Flow happy)
// ────────────────────────────────────────────────────────────────────────────

const NODE_TYPES: NodeTypes = {
  step: StepNode as unknown as NodeTypes['step'],
}

// ────────────────────────────────────────────────────────────────────────────
// Dagre auto-layout
// ────────────────────────────────────────────────────────────────────────────

function layoutWithDagre(nodes: Node[], edges: Edge[]): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({
    rankdir: 'TB',
    ranksep: DAGRE_RANK_SEP,
    nodesep: DAGRE_NODE_SEP,
    marginx: 20,
    marginy: 20,
  })

  nodes.forEach((n) => g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT }))
  edges.forEach((e) => g.setEdge(e.source, e.target))

  dagre.layout(g)

  const laidOutNodes = nodes.map((n) => {
    const pos = g.node(n.id)
    return {
      ...n,
      position: {
        x: pos.x - NODE_WIDTH / 2,
        y: pos.y - NODE_HEIGHT / 2,
      },
    }
  })

  return { nodes: laidOutNodes, edges }
}

// ────────────────────────────────────────────────────────────────────────────
// Build nodes + edges from steps
// ────────────────────────────────────────────────────────────────────────────

function buildGraph(
  steps: ProcessStep[],
  primaryRepo: string,
): { nodes: Node[]; edges: Edge[] } {
  const rawNodes: Node[] = steps.map((step) => ({
    id: step.entity_id,
    type: 'step',
    position: { x: 0, y: 0 },
    data: {
      label: step.label,
      step_kind: step.step_kind,
      repo: step.repo,
      primaryRepo,
      step_index: step.step_index,
      source_file: step.source_file,
      start_line: step.start_line,
      entity_id: step.entity_id,
    } satisfies StepNodeData,
  }))

  const rawEdges: Edge[] = steps.slice(1).map((step, idx) => {
    const prev = steps[idx]
    const isCross = step.repo !== prev.repo
    return {
      id: `e-${prev.entity_id}-${step.entity_id}`,
      source: prev.entity_id,
      target: step.entity_id,
      animated: isCross,
      style: {
        stroke: isCross ? '#a78bfa' : '#475569',
        strokeWidth: 1.5,
        strokeDasharray: isCross ? '5 3' : undefined,
      },
      label: step.edge_kind,
      labelStyle: { fill: '#64748b', fontSize: 9 },
      labelBgStyle: { fill: 'transparent' },
    }
  })

  return layoutWithDagre(rawNodes, rawEdges)
}

// ────────────────────────────────────────────────────────────────────────────
// FitView controller — fires once on mount and whenever steps change
// ────────────────────────────────────────────────────────────────────────────

function FitOnMount({ deps }: { deps: unknown }) {
  const { fitView } = useReactFlow()
  useEffect(() => {
    const t = window.setTimeout(() => fitView(FIT_VIEW_OPTIONS), 50)
    return () => window.clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deps, fitView])
  return null
}

// ────────────────────────────────────────────────────────────────────────────
// Main component
// ────────────────────────────────────────────────────────────────────────────

interface FlowDagProps {
  steps: ProcessStep[]
  primaryRepo: string
  onStepClick?: (step: ProcessStep) => void
  /** Jarvis hook reserve — set of entity_id strings to highlight (#1157) */
  highlightedEntityIds?: Set<string>
}

export function FlowDag({ steps, primaryRepo, onStepClick }: FlowDagProps) {
  const { nodes: initialNodes, edges: initialEdges } = useMemo(
    () => (steps.length > 0 ? buildGraph(steps, primaryRepo) : { nodes: [], edges: [] }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [steps.map((s) => s.entity_id).join(','), primaryRepo],
  )

  const [nodes, , onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)

  // Re-sync when initialNodes change (step list updated)
  // useNodesState doesn't auto-reset on prop change — replace all nodes explicitly
  useEffect(() => {
    const replaceChanges: NodeReplaceChange<Node>[] = initialNodes.map((n) => ({
      type: 'replace',
      id: n.id,
      item: n,
    }))
    if (replaceChanges.length > 0) onNodesChange(replaceChanges)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialNodes])

  useEffect(() => {
    setEdges(initialEdges)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialEdges])

  const onConnect: OnConnect = useCallback(
    (params) => setEdges((eds) => addEdge(params, eds)),
    [setEdges],
  )

  const handleNodeClick = useCallback(
    (_event: React.MouseEvent, node: Node) => {
      if (!onStepClick) return
      const step = steps.find((s) => s.entity_id === node.id)
      if (step) onStepClick(step)
    },
    [steps, onStepClick],
  )

  if (steps.length === 0) {
    return (
      <div className="flex items-center justify-center h-40 text-slate-500 text-sm">
        No steps to visualize
      </div>
    )
  }

  return (
    <div
      className="w-full rounded-lg overflow-hidden border border-slate-800"
      style={{ height: Math.min(Math.max(steps.length * 80 + 80, 220), 520) }}
      data-testid="flow-dag"
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={handleNodeClick}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={FIT_VIEW_OPTIONS}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        deleteKeyCode={null}
        colorMode="dark"
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={16}
          size={1}
          color="#1e293b"
        />
        <Controls showInteractive={false} position="bottom-right" />
        <MiniMap
          nodeColor={(n) => stepKindSpec((n.data as StepNodeData).step_kind).hex}
          maskColor="rgba(0,0,0,0.6)"
          position="top-right"
          pannable
          zoomable
        />
        <FitOnMount deps={initialNodes} />
      </ReactFlow>
    </div>
  )
}
