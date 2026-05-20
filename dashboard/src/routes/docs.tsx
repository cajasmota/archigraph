import { useParams } from 'react-router-dom'
import { DocsPage } from '@/components/docs/DocsPage'

/**
 * Surface 5 — Docs Portal route.
 *
 * URL patterns (from App.tsx):
 *   /docs/:group         → no doc selected, show empty state
 *   /docs/:group/*       → wildcard captures doc path (e.g. "acme-web/modules/auth/overview")
 */
export function DocsRoute() {
  const { group = 'fixture-a', '*': docPathWildcard } = useParams<{
    group: string
    '*': string
  }>()

  // Wildcard may be empty string when just at /docs/:group
  const docPath = docPathWildcard || undefined

  return <DocsPage group={group} docPath={docPath} />
}
