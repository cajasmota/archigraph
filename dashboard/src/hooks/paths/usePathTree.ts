import { usePathList } from './usePathList'
import type { PathTreeNode } from '@/types/api'

/**
 * Returns the prefix tree from the cached path list response.
 * No extra fetch — derived from the `tree` field already in the React Query cache.
 */
export function usePathTree(group: string): {
  tree: PathTreeNode[]
  isLoading: boolean
} {
  const { data, isLoading } = usePathList(group, { page_size: 50 })
  return {
    tree: data?.tree ?? [],
    isLoading,
  }
}
