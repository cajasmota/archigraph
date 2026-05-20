import { create } from 'zustand'

/**
 * Zustand slice for the 3D graph camera.
 * High-frequency state (zoom, hover) that should NOT live in URL params.
 *
 * The `graphRef` is set by <GraphCanvas3D> once the instance is mounted,
 * allowing toolbar / inspector to call camera methods imperatively.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type ForceGraphInstance = any  // 3d-force-graph has no @types package

interface GraphCameraState {
  graphRef: ForceGraphInstance | null
  zoomLevel: number
  hoveredNodeId: string | null

  // Actions
  setGraphRef: (ref: ForceGraphInstance | null) => void
  setZoomLevel: (z: number) => void
  setHoveredNode: (id: string | null) => void
  zoomToNode: (nodeId: string) => void
  resetView: () => void
}

export const useGraphCameraStore = create<GraphCameraState>((set, get) => ({
  graphRef: null,
  zoomLevel: 1.0,
  hoveredNodeId: null,

  setGraphRef: (ref) => set({ graphRef: ref }),
  setZoomLevel: (z) => set({ zoomLevel: z }),
  setHoveredNode: (id) => set({ hoveredNodeId: id }),

  zoomToNode: (nodeId) => {
    const { graphRef } = get()
    if (!graphRef) return
    // 3d-force-graph API: centerAt + zoom
    const node = graphRef.graphData().nodes.find((n: { id: string }) => n.id === nodeId)
    if (!node) return
    const distance = 80
    const distRatio = 1 + distance / Math.hypot(node.x ?? 0, node.y ?? 0, node.z ?? 0)
    graphRef.cameraPosition(
      {
        x: (node.x ?? 0) * distRatio,
        y: (node.y ?? 0) * distRatio,
        z: (node.z ?? 0) * distRatio,
      },
      node,
      800, // ms transition
    )
  },

  resetView: () => {
    const { graphRef } = get()
    if (!graphRef) return
    graphRef.zoomToFit(600)
  },
}))

/** Convenience selector hooks */
export const useGraphRef = () => useGraphCameraStore((s) => s.graphRef)
export const useZoomLevel = () => useGraphCameraStore((s) => s.zoomLevel)
export const useHoveredNodeId = () => useGraphCameraStore((s) => s.hoveredNodeId)
