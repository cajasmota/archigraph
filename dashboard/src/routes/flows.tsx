import { useParams } from 'react-router-dom'
import { Workflow } from 'lucide-react'
import { EmptyState } from '@/components/shared/EmptyState'

export function FlowsRoute() {
  const { group } = useParams<{ group: string }>()
  return (
    <div className="h-full flex flex-col items-center justify-center">
      <EmptyState
        icon={Workflow}
        title="Process Flow Explorer (Surface 2)"
        message={`Flow chain explorer for group "${group}" — coming in M2 phase 2.`}
      />
    </div>
  )
}
