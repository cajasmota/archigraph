import { KindIcon } from '@/components/shared/KindIcon'
import { kindColors } from '@/lib/colors'
import type { EntityKind } from '@/types/api'

interface NodeChipProps {
  kind: EntityKind
  label: string
  repo?: string
  onClick?: () => void
  className?: string
}

/**
 * Small badge: kind icon + entity label.
 * Reused in EntityInspector neighbor list and GraphSearchTypeahead results.
 */
export function NodeChip({ kind, label, repo, onClick, className = '' }: NodeChipProps) {
  const colors = kindColors(kind)
  const Tag = onClick ? 'button' : 'span'
  return (
    <Tag
      type={onClick ? 'button' : undefined}
      onClick={onClick}
      className={[
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium',
        'border border-transparent transition-colors',
        colors.bg,
        colors.text,
        onClick ? 'cursor-pointer hover:opacity-80 focus-visible:ring-1 focus-visible:ring-sky-400 focus-visible:outline-none' : '',
        className,
      ].join(' ')}
      title={repo ? `${label} (${repo})` : label}
    >
      <KindIcon kind={kind} className="w-3 h-3 shrink-0" />
      <span className="truncate max-w-[160px]">{label}</span>
    </Tag>
  )
}
