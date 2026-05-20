import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { fetchGraph } from '@/api/client'
import { useGraphLoD } from './useGraphLoD'
import type { GraphFilters, GraphNode, GraphEdge, Community, LodLevel, ServerLod } from '@/types/api'
import type { ZoomLevel, Viewport } from './useGraphLoD'

// ── LOD tier selection ────────────────────────────────────────────────────────
// Map the camera zoom level to a server-side LOD tier.  The server caps the
// "full" tier at 20 000 nodes; for large groups we must start at "centroids"
// and step up as the user zooms in.
const LOD_ZOOM_OUT_THRESHOLD = 0.5
const LOD_MID_THRESHOLD = 2.0

function zoomToServerLod(zoom: ZoomLevel): ServerLod {
  if (zoom < LOD_ZOOM_OUT_THRESHOLD) return 'centroids'
  if (zoom < LOD_MID_THRESHOLD) return 'mid'
  return 'full'
}

// Synthetic edge kinds emitted by the server only for layout purposes.
// These should not appear as user-facing filter chips.
const SYNTHETIC_KINDS = new Set<string>(['COMMUNITY_LINK'])

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
 * Fetches the graph for a group, then optionally applies client-side LoD
 * culling via useGraphLoD.
 *
 * When we send a server-side LOD param the server has already filtered the
 * node set to the appropriate tier; client-side re-filtering would remove
 * nodes that don't match the client's heuristic (e.g. centroid nodes have
 * is_centroid=true but the mid-zoom heuristic looks for god-node IDs in
 * community top_entities, which wouldn't match centroid IDs).  In server-LOD
 * mode we therefore show all returned nodes and skip the useGraphLoD filter.
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
  // Derive the server-side LOD from the current zoom level.  This is passed
  // as a query param so the server returns only the nodes appropriate for the
  // current view (avoids fetching 20k+ nodes when zoomed out to centroid tier).
  const serverLod = zoomToServerLod(zoomLevel)

  const {
    data,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['graph', group, filters.repo, serverLod],
    queryFn: () => fetchGraph(group, { repo: filters.repo, lod: serverLod }),
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

  // The server already performed LOD selection — trust it and show all
  // returned nodes rather than re-filtering client-side.  useGraphLoD is
  // still called to derive the LodLevel enum value (used for the LoD
  // indicator UI) and to apply selected-node 1-hop expansion, but we bypass
  // its visibility filter and show all server-returned nodes directly.
  const { lodLevel } = useGraphLoD(
    data?.nodes ?? [],
    filteredEdges,
    data?.communities ?? [],
    zoomLevel,
    viewport,
    selectedNodeId,
  )

  // For the "blocked" sentinel the server returns 0 nodes; surface that as-is.
  const serverLodLevel: LodLevel = data?.lod ?? lodLevel

  // Show all nodes the server returned (already LOD-filtered).
  // Apply only the edge-kind filter on top.
  const nodes = useMemo<GraphNode[]>(() => {
    if (!data) return []
    return data.nodes
  }, [data])

  const edges = useMemo<GraphEdge[]>(() => {
    if (!data) return []
    return filteredEdges
  }, [data, filteredEdges])

  // Exclude synthetic centroid-tier edges (COMMUNITY_LINK) from the filter
  // chip bar — they are internal layout hints and not meaningful to the user.
  const allEdgeKinds = useMemo(() => {
    if (!data) return []
    return [...new Set(data.edges.map((e) => e.kind))].filter(
      (k) => !SYNTHETIC_KINDS.has(k),
    )
  }, [data])

  return {
    nodes,
    edges,
    communities: data?.communities ?? [],
    allEdgeKinds,
    lodLevel: serverLodLevel,
    totalNodeCount: data?.total_node_count ?? 0,
    isLoading,
    error: error as Error | null,
    refetch,
  }
}
