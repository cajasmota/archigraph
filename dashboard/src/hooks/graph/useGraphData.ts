import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { fetchGraph } from '@/api/client'
import { useGraphLoD } from './useGraphLoD'
import type { GraphFilters, GraphNode, GraphEdge, Community, LodLevel } from '@/types/api'
import type { ZoomLevel, Viewport } from './useGraphLoD'

export interface GraphDataResult {
  /** Filtered node array for the graph renderer */
  nodes: GraphNode[]
  /** Filtered edge array for the graph renderer */
  edges: GraphEdge[]
  /** All communities */
  communities: Community[]
  /** All unique edge kinds present in the full graph */
  allEdgeKinds: string[]
  /** Current LoD level */
  lodLevel: LodLevel
  /** Total node count (unfiltered, for cap display) */
  totalNodeCount: number
  isLoading: boolean
  error: Error | null
  refetch: () => void
}

/**
 * Fetches the graph for a group, then derives visible nodes/edges
 * via useGraphLoD. Both 3D and 2D canvas components consume this.
 *
 * @param group - group ID
 * @param filters - edge-kind and repo filters
 * @param zoomLevel - current camera zoom (drives LoD tier)
 * @param viewport - frustum bounds for zoom-in culling (null = no cull)
 * @param selectedNodeId - always visible regardless of LoD
 */
export function useGraphData(
  group: string,
  filters: GraphFilters,
  zoomLevel: ZoomLevel,
  viewport: Viewport | null,
  selectedNodeId: string | null,
): GraphDataResult {
  const {
    data,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['graph', group, filters.repo],
    queryFn: () => fetchGraph(group, { repo: filters.repo }),
    staleTime: 5 * 60 * 1000,
    enabled: !!group,
  })

  // Apply edge-kind filter client-side (cheap — just a Set lookup)
  const filteredEdges = useMemo(() => {
    if (!data) return []
    if (!filters.edge_kinds || filters.edge_kinds.length === 0) return data.edges
    const kinds = new Set(filters.edge_kinds)
    return data.edges.filter((e) => kinds.has(e.kind))
  }, [data, filters.edge_kinds])

  // Derive LoD visibility
  const { visibleNodeIds, visibleEdgeIds, lodLevel } = useGraphLoD(
    data?.nodes ?? [],
    filteredEdges,
    data?.communities ?? [],
    zoomLevel,
    viewport,
    selectedNodeId,
  )

  // Materialize filtered arrays for the renderers
  const nodes = useMemo(() => {
    if (!data) return []
    return data.nodes.filter((n) => visibleNodeIds.has(n.id))
  }, [data, visibleNodeIds])

  const edges = useMemo(() => {
    if (!data) return []
    return filteredEdges.filter((e) => visibleEdgeIds.has(e.id))
  }, [filteredEdges, visibleEdgeIds])

  const allEdgeKinds = useMemo(() => {
    if (!data) return []
    return [...new Set(data.edges.map((e) => e.kind))]
  }, [data])

  return {
    nodes,
    edges,
    communities: data?.communities ?? [],
    allEdgeKinds,
    lodLevel,
    totalNodeCount: data?.total_node_count ?? 0,
    isLoading,
    error: error as Error | null,
    refetch,
  }
}
