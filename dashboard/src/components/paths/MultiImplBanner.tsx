import { Info } from 'lucide-react'

interface MultiImplBannerProps {
  count: number
  path: string
}

/**
 * Shown in PathDetailPage when fetches_count > 1 —
 * i.e. when there are multiple handler implementations behind a single path.
 */
export function MultiImplBanner({ count, path }: MultiImplBannerProps) {
  if (count <= 1) return null
  return (
    <div
      className="flex items-start gap-2 px-4 py-2.5 bg-amber-950/40 border-b border-amber-800 text-sm"
      role="note"
      aria-label="Multiple implementations"
    >
      <Info className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" aria-hidden />
      <span className="text-amber-300">
        <strong>{count} handler implementations</strong> found for{' '}
        <code className="font-mono text-amber-200">{path}</code>. Responses may
        vary by verb or route variant.
      </span>
    </div>
  )
}
