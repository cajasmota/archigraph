/**
 * Lazy-loaded wrapper for FlowDag.
 *
 * React.lazy() + Suspense ensures the @xyflow/react chunk (~200KB) is NOT
 * included in the main bundle. The chunk is fetched only when the flow detail
 * panel is first opened.
 */
import { lazy, Suspense } from 'react'
import type { ComponentProps } from 'react'
import type { FlowDag as FlowDagType } from './FlowDag'

const FlowDagImpl = lazy(() =>
  import('./FlowDag').then((m) => ({ default: m.FlowDag })),
)

type FlowDagProps = ComponentProps<typeof FlowDagType>

export function FlowDagLazy(props: FlowDagProps) {
  return (
    <Suspense
      fallback={
        <div
          className="flex items-center justify-center h-40 text-slate-500 text-sm"
          data-testid="flow-dag-loading"
        >
          Loading DAG…
        </div>
      }
    >
      <FlowDagImpl {...props} />
    </Suspense>
  )
}
