import {cn} from '../../lib/cn'
import {hubPresentation, type HubTone} from '../../lib/hubPresentation'
import type {statemachine} from '../../../wailsjs/go/models'

// Tone → text color. The ring, glow, and icon all draw from currentColor, so
// setting the tone once on the wrapper drives the whole hub's color.
const toneText: Record<HubTone, string> = {
    muted: 'text-text-muted',
    accent: 'text-accent',
    success: 'text-success',
    warning: 'text-warning',
    danger: 'text-danger',
}

const R = 94
const CIRC = 2 * Math.PI * R
// Busy arc covers ~28% of the ring and spins; the settled ring is a full loop.
const BUSY_DASH = `${CIRC * 0.28} ${CIRC * 0.72}`

/**
 * The focal point: one large ring whose color, icon, and label reflect the
 * live backend status and nothing else. Transitional states spin a bright arc;
 * settled states show a full ring with a soft breathing glow. It renders the
 * status projection only — it triggers no actions and makes no optimistic
 * claims (see hubPresentation).
 */
export function StatusHub({status}: {status: statemachine.Status | null}) {
    const {tone, Icon, label, sublabel, busy} = hubPresentation(status)
    const active = tone === 'success'
    const lit = tone !== 'muted'

    return (
        <div className="flex flex-col items-center gap-5">
            <div className={cn('relative grid size-52 place-items-center', toneText[tone])}>
                {/* Soft glow behind the ring — only when the state is lit. */}
                {lit && (
                    <div
                        className={cn(
                            'pointer-events-none absolute inset-3 rounded-full bg-current opacity-40 blur-2xl',
                            (active || busy) && 'animate-breathe motion-reduce:animate-none',
                        )}
                        aria-hidden
                    />
                )}

                <svg viewBox="0 0 208 208" className="absolute inset-0 size-full -rotate-90" aria-hidden>
                    {/* Faint inner disc for depth. */}
                    <circle cx="104" cy="104" r="84" className="fill-current opacity-[0.05]" />
                    {/* Track. */}
                    <circle
                        cx="104"
                        cy="104"
                        r={R}
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="8"
                        className="opacity-15"
                    />
                    {/* Progress / status arc. */}
                    <circle
                        cx="104"
                        cy="104"
                        r={R}
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="8"
                        strokeLinecap="round"
                        strokeDasharray={busy ? BUSY_DASH : undefined}
                        className={cn(busy && 'animate-ring-spin motion-reduce:animate-none')}
                        style={busy ? {transformOrigin: 'center'} : undefined}
                    />
                </svg>

                <Icon className={cn('relative size-14', busy && 'animate-pulse-soft motion-reduce:animate-none')} aria-hidden />
            </div>

            <div className="flex flex-col items-center gap-1 text-center">
                <h1 className="font-display text-2xl font-semibold tracking-tight text-text">{label}</h1>
                <p className="max-w-xs text-sm text-text-secondary">{sublabel}</p>
            </div>
        </div>
    )
}
