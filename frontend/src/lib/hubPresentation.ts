import {AlertTriangle, Loader2, Power, Zap, type LucideIcon} from 'lucide-react'
import type {statemachine} from '../../wailsjs/go/models'

export type HubTone = 'muted' | 'accent' | 'success' | 'warning' | 'danger'

export interface HubPresentation {
    tone: HubTone
    Icon: LucideIcon
    label: string
    sublabel: string
    /** Transitional state — the ring shows an animated rotating arc. */
    busy: boolean
}

function fastTarget(mode: string | undefined): string {
    switch (mode) {
        case 'discord':
            return 'Discord'
        case 'custom':
            return 'Custom domains'
        case 'full':
            return 'All HTTPS traffic'
        default:
            return 'DPI bypass'
    }
}

/**
 * Maps the unified backend status to how the central hub should look. This is
 * a pure projection of reality — it never anticipates a request that hasn't
 * landed as a real status yet (the "state always reflects reality" rule). Order
 * matters: the error state wins over the nominal Fast Mode / Off states.
 */
export function hubPresentation(status: statemachine.Status | null): HubPresentation {
    if (!status) {
        return {tone: 'muted', Icon: Loader2, label: 'Checking…', sublabel: 'Reading current state', busy: true}
    }

    if (status.state === 'error') {
        return {
            tone: 'danger',
            Icon: AlertTriangle,
            label: 'Error',
            sublabel: status.error || 'Something went wrong',
            busy: false,
        }
    }

    const fast = status.fastStatus
    if (status.subMode === 'fast') {
        const starting =
            status.transitioning || fast?.state === 'starting' || fast?.state === 'restarting'
        if (starting) {
            return {tone: 'accent', Icon: Zap, label: 'Starting…', sublabel: 'Bringing up DPI bypass', busy: true}
        }
        if (status.state === 'fast-active') {
            return {
                tone: 'success',
                Icon: Zap,
                label: 'Fast Mode active',
                sublabel: fastTarget(fast?.mode),
                busy: false,
            }
        }
    }

    // A generic transition (e.g. tearing the mode down) with no sub-mode claimed.
    if (status.transitioning) {
        return {tone: 'accent', Icon: Loader2, label: 'Working…', sublabel: 'Applying your change', busy: true}
    }

    return {tone: 'muted', Icon: Power, label: 'Off', sublabel: 'Not protected', busy: false}
}
