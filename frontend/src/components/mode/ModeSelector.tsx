import {Power, ShieldCheck, Zap} from 'lucide-react'
import {fastPresentation, privatePresentation} from '../../lib/statusPresentation'
import type {PanelKey} from '../../lib/types'
import {useAppState} from '../../state/useAppState'
import {SegmentedControl, type SegmentedOption} from '../ui/SegmentedControl'
import {StatusDot} from '../ui/StatusDot'

interface ModeSelectorProps {
    panel: PanelKey
    onPanelChange: (panel: PanelKey) => void
}

/**
 * Both navigation (which panel is shown) and a live status display (which
 * mode is actually running) — deliberately decoupled. Clicking a segment
 * only changes which panel you're looking at; the dots reflect ground truth
 * from the backend regardless of which panel that is.
 */
export function ModeSelector({panel, onPanelChange}: ModeSelectorProps) {
    const {status} = useAppState()
    const fast = fastPresentation(status)
    const priv = privatePresentation(status)

    const options: SegmentedOption<PanelKey>[] = [
        {value: 'off', label: 'Off', icon: <Power className="size-4" aria-hidden />},
        {
            value: 'fast',
            label: 'Fast',
            icon: <Zap className="size-4" aria-hidden />,
            indicator: fast.tone !== 'neutral' ? <StatusDot tone={fast.tone} pulse={fast.pulse} /> : undefined,
        },
        {
            value: 'private',
            label: 'Private',
            icon: <ShieldCheck className="size-4" aria-hidden />,
            indicator: priv.tone !== 'neutral' ? <StatusDot tone={priv.tone} pulse={priv.pulse} /> : undefined,
        },
    ]

    return (
        <div className="flex flex-col items-center gap-3 py-2">
            <SegmentedControl aria-label="Mode" value={panel} onValueChange={onPanelChange} options={options} />
        </div>
    )
}
