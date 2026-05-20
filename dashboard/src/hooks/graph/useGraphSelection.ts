import { useSearchParams } from 'react-router-dom'
import { useCallback } from 'react'

const PARAM = 'selected'

/**
 * Reads/writes the selected node ID from URL hash (via search param).
 * Provides shareable deep-links: /graph/acme?selected=acme-web::OrderSvc::42
 */
export function useGraphSelection(): {
  selectedNodeId: string | null
  select: (id: string | null) => void
  clear: () => void
} {
  const [params, setParams] = useSearchParams()
  const selectedNodeId = params.get(PARAM)

  const select = useCallback(
    (id: string | null) => {
      setParams((prev) => {
        const next = new URLSearchParams(prev)
        if (id === null || id === '') {
          next.delete(PARAM)
        } else {
          next.set(PARAM, id)
        }
        return next
      })
    },
    [setParams],
  )

  const clear = useCallback(() => select(null), [select])

  return { selectedNodeId, select, clear }
}
