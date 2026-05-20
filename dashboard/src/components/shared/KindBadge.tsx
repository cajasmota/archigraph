import { KindIcon } from './KindIcon'
import { kindColors } from '@/lib/colors'
import type { EntityKind } from '@/types/api'

interface KindBadgeProps {
  kind: EntityKind
  className?: string
}

export function KindBadge({ kind, className = '' }: KindBadgeProps) {
  const colors = kindColors(kind)
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium border border-transparent ${colors.bg} ${colors.text} ${className}`}
      aria-label={`Kind: ${kind}`}
    >
      <KindIcon kind={kind} className="w-3 h-3" />
      {kind}
    </span>
  )
}
