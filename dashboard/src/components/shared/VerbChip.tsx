import { VERB_COLORS } from '@/lib/colors'
import type { HttpVerb } from '@/types/api'

interface VerbChipProps {
  verb: HttpVerb
  onClick?: () => void
  active?: boolean
  className?: string
}

export function VerbChip({ verb, onClick, active = false, className = '' }: VerbChipProps) {
  const colors = VERB_COLORS[verb]
  const baseClasses = [
    'inline-flex items-center px-2 py-0.5 rounded text-xs font-bold font-mono border',
    'transition-opacity',
    colors.bg,
    colors.text,
    colors.border,
    active ? 'ring-1 ring-current' : '',
    onClick ? 'cursor-pointer hover:opacity-80' : '',
    className,
  ].filter(Boolean).join(' ')

  if (onClick) {
    return (
      <button
        type="button"
        className={baseClasses}
        onClick={onClick}
        aria-pressed={active}
        aria-label={`Filter by ${verb}`}
      >
        {verb}
      </button>
    )
  }

  return (
    <span className={baseClasses} aria-label={verb}>
      {verb}
    </span>
  )
}
