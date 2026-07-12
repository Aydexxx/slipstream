import {cn} from '../../lib/cn'

export type StatusTone = 'neutral' | 'success' | 'warning' | 'danger'

const toneClasses: Record<StatusTone, string> = {
    neutral: 'bg-text-muted',
    success: 'bg-success',
    warning: 'bg-warning',
    danger: 'bg-danger',
}

export function StatusDot({tone, pulse = false}: {tone: StatusTone; pulse?: boolean}) {
    return (
        <span
            className={cn('inline-block size-2 shrink-0 rounded-full', toneClasses[tone], pulse && 'animate-pulse-soft')}
            aria-hidden
        />
    )
}
