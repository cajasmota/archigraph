import { useParams } from 'react-router-dom'
import { BookOpen } from 'lucide-react'
import { EmptyState } from '@/components/shared/EmptyState'

export function DocsRoute() {
  const { group } = useParams<{ group: string }>()
  return (
    <div className="h-full flex flex-col items-center justify-center">
      <EmptyState
        icon={BookOpen}
        title="Docs Portal (Surface 5)"
        message={`Documentation portal for group "${group}" — coming in M2 phase 5.`}
      />
    </div>
  )
}
