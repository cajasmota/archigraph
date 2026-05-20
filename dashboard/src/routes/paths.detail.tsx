import { useParams } from 'react-router-dom'
import { PathDetailPage } from '@/components/paths/PathDetailPage'

export function PathsDetailRoute() {
  const { group = 'fixture-a' } = useParams<{ group: string }>()
  return <PathDetailPage group={group} />
}
