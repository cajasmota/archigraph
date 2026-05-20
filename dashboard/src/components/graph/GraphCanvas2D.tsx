import { useRef, useCallback, useEffect, memo } from 'react'
import ForceGraph2D from 'react-force-graph-2d'
import { communityColor } from '@/hooks/graph/useCommunityColors'
import { edgeKindColor } from './EdgeBadge'
import type { GraphNode, GraphEdge } from '@/types/api'
import { useGraphCameraStore } from '@/store/graphCameraStore'

interface GraphCanvas2DProps {
  nodes: GraphNode[]
  edges: GraphEdge[]
  selectedNodeId: string | null
  hoveredNodeId: string | null
  onNodeClick: (node: GraphNode) => void
  onNodeHover: (node: GraphNode | null) => void
  onZoomChange?: (zoom: number) => void
  highContrast?: boolean
  className?: string
}

const NODE_BASE_R = 4
const CENTROID_SCALE = 3.0
const PAGERANK_SCALE = 12.0

/**
 * 2D force-graph fallback. Same prop interface as GraphCanvas3D.
 * Used for:
 * - prefers-reduced-motion (no WebGL spin)
 * - low-end devices (2D canvas is ~5x cheaper than 3D WebGL)
 * - explicit user toggle
 */
const GraphCanvas2DInner = ({
  nodes,
  edges,
  selectedNodeId,
  hoveredNodeId,
  onNodeClick,
  onNodeHover,
  onZoomChange,
  highContrast = false,
  className = '',
}: GraphCanvas2DProps) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const { setZoomLevel } = useGraphCameraStore()

  const nodeColor = useCallback((n: GraphNode) => {
    if (n.id === selectedNodeId) return '#38bdf8'
    if (n.id === hoveredNodeId) return '#e2e8f0'
    return communityColor(n.community_id ?? 0)
  }, [selectedNodeId, hoveredNodeId])

  const nodeRelSize = useCallback((n: GraphNode) => {
    if (n.is_centroid) return (n.centroid_size ?? 100) / 50 * CENTROID_SCALE
    return NODE_BASE_R + (n.pagerank ?? 0) * PAGERANK_SCALE
  }, [])

  const linkColor = useCallback((e: GraphEdge) => {
    const base = edgeKindColor(e.kind)
    return highContrast ? base : base + '99'
  }, [highContrast])

  const handleZoom = useCallback(({ k }: { k: number }) => {
    setZoomLevel(k)
    onZoomChange?.(k)
  }, [setZoomLevel, onZoomChange])

  return (
    <div
      ref={containerRef}
      className={['w-full h-full', className].join(' ')}
      aria-label="2D dependency graph"
      role="img"
      aria-describedby="graph-2d-a11y-desc"
    >
      <span id="graph-2d-a11y-desc" className="sr-only">
        Interactive 2D force-directed graph. Use the inspector panel to navigate nodes with keyboard.
      </span>
      <ForceGraph2D
        graphData={{
          nodes: nodes.map((n) => ({ ...n })),
          links: edges.map((e) => ({ ...e, source: e.source, target: e.target })),
        }}
        backgroundColor="#020617"
        nodeColor={nodeColor}
        nodeRelSize={nodeRelSize}
        linkColor={linkColor}
        linkWidth={highContrast ? 1.5 : 0.8}
        onNodeClick={(n) => onNodeClick(n as GraphNode)}
        onNodeHover={(n) => onNodeHover(n as GraphNode | null)}
        onZoom={handleZoom}
        cooldownTime={2000}
        d3AlphaDecay={0.02}
        d3VelocityDecay={0.3}
        width={containerRef.current?.clientWidth ?? 800}
        height={containerRef.current?.clientHeight ?? 600}
      />
    </div>
  )
}

export const GraphCanvas2D = memo(GraphCanvas2DInner)
