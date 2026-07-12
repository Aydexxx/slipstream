import {formatRelativeTime, isZeroTime} from '../../lib/format'
import {StatusDot} from '../ui/StatusDot'

export function HandshakeIndicator({lastHandshake, ageSec}: {lastHandshake: string; ageSec: number}) {
    const fresh = !isZeroTime(lastHandshake) && ageSec >= 0 && ageSec < 180
    return (
        <div className="flex items-center gap-2 text-sm">
            <StatusDot tone={fresh ? 'success' : 'neutral'} />
            <span className="text-text-secondary">Last handshake:</span>
            <span className="text-text">{formatRelativeTime(lastHandshake)}</span>
        </div>
    )
}
