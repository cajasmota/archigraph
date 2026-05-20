import { useParams } from 'react-router-dom'
import { Network } from 'lucide-react'
import { EmptyState } from '@/components/shared/EmptyState'

export function GraphRoute() {
  const { group } = useParams<{ group: string }>()
  return (
    <div className="h-full flex flex-col items-center justify-center">
      <EmptyState
        icon={Network}
        title="Graph Viewer (Surface 1)"
        message={`3D force-directed graph for group "${group}" — milestone 2 backend work in progress.`}
      />
    </div>
  )
}
