import { useQuery } from '@tanstack/react-query'
import { fetchTopicDetail } from '@/api/client'
import type { TopicDetailV2 } from '@/types/api'

/**
 * Fetches the rich per-topic detail payload from the v2 endpoint (#1138).
 * Enabled only when both group and topicId are non-empty.
 */
export function useTopicDetailFetch(
  group: string | null | undefined,
  topicId: string | null | undefined,
) {
  return useQuery<TopicDetailV2, Error>({
    queryKey: ['topology-topic-detail', group, topicId],
    queryFn: () => fetchTopicDetail(group!, topicId!),
    enabled: !!(group && topicId),
    staleTime: 60 * 1000,
    placeholderData: (prev) => prev,
  })
}
