import type {statemachine} from '../../wailsjs/go/models'
import type {StatusTone} from '../components/ui/StatusDot'

export interface StatusPresentation {
    tone: StatusTone
    pulse: boolean
    label: string
}

/** Live status for the Fast Mode segment — only lit while Fast Mode is the active sub-mode. */
export function fastPresentation(status: statemachine.Status | null): StatusPresentation {
    if (!status || status.subMode !== 'fast') return {tone: 'neutral', pulse: false, label: 'Off'}
    if (status.state === 'error') return {tone: 'danger', pulse: false, label: 'Error'}
    if (status.transitioning) return {tone: 'warning', pulse: true, label: 'Starting…'}
    return {tone: 'success', pulse: false, label: 'Active'}
}

/** Live status for the Private Mode segment — only lit while Private Mode is the active sub-mode. */
export function privatePresentation(status: statemachine.Status | null): StatusPresentation {
    if (!status || status.subMode !== 'private') return {tone: 'neutral', pulse: false, label: 'Off'}
    if (status.state === 'error') return {tone: 'danger', pulse: false, label: 'Error'}
    if (status.state === 'private-connecting') return {tone: 'warning', pulse: true, label: 'Connecting…'}
    if (status.state === 'private-active') return {tone: 'success', pulse: false, label: 'Connected'}
    return {tone: 'neutral', pulse: false, label: 'Off'}
}
