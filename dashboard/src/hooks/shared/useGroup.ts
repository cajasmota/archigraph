import { useRegistry } from './useRegistry'
import type { GroupMeta } from '@/types/api'

export function useGroup(groupId: string): GroupMeta | undefined {
  const { data } = useRegistry()
  return data?.groups.find((g) => g.id === groupId)
}
