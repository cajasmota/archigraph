import { useParams } from 'react-router-dom'
import { Radio } from 'lucide-react'
import { EmptyState } from '@/components/shared/EmptyState'

export function TopologyRoute() {
  const { group } = useParams<{ group: string }>()
  return (
    <div className="h-full flex flex-col items-center justify-center">
      <EmptyState
        icon={Radio}
        title="Broker Topology (Surface 3)"
        message={`Message broker topology for group "${group}" — coming in M2 phase 3.`}
      />
    </div>
  )
}
