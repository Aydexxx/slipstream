import {AlertTriangle, Loader2, Power, ShieldAlert, ShieldCheck, ShieldHalf, Zap, type LucideIcon} from 'lucide-react'
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
 * Maps the single unified backend status to how the central hub should look.
 * This is a pure projection of reality — it never anticipates a request that
 * hasn't landed as a real status yet (the "state always reflects reality"
 * rule). Order matters: the most safety-critical conditions (error, then a
 * kill switch blocking traffic) win over the nominal mode states.
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

    // Kill switch holding traffic closed while not actually connected: the user
    // is offline-by-design and needs to know. Mirrors useTrayNotifications.
    if (status.killSwitchArmed && status.state !== 'private-active') {
        return {
            tone: 'danger',
            Icon: ShieldAlert,
            label: 'Kill switch engaged',
            sublabel: 'Traffic blocked until the tunnel reconnects',
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

    if (status.state === 'private-connecting') {
        return {
            tone: 'warning',
            Icon: ShieldHalf,
            label: 'Connecting…',
            sublabel: 'Establishing the secure tunnel',
            busy: true,
        }
    }

    if (status.state === 'private-active') {
        const host = status.privateStatus?.endpoint || ''
        return {
            tone: 'success',
            Icon: ShieldCheck,
            label: 'Private Mode active',
            sublabel: host ? `Encrypted tunnel · ${host}` : 'Encrypted full tunnel',
            busy: false,
        }
    }

    // A generic transition (e.g. tearing a mode down) with no sub-mode claimed.
    if (status.transitioning) {
        return {tone: 'accent', Icon: Loader2, label: 'Working…', sublabel: 'Applying your change', busy: true}
    }

    return {tone: 'muted', Icon: Power, label: 'Off', sublabel: 'Not protected', busy: false}
}
