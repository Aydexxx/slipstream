import type {fastmode} from '../../../wailsjs/go/models'
import {formatDurationSince} from '../../lib/format'
import type {FastMode} from '../../state/AppStateContext'
import {SegmentedControl, type SegmentedOption} from '../ui/SegmentedControl'
import {CustomDomainEditor} from './CustomDomainEditor'
import {StrategySelector} from './StrategySelector'

const SUBMODE_OPTIONS: SegmentedOption<FastMode>[] = [
    {value: 'full', label: 'Full'},
    {value: 'discord', label: 'Discord'},
    {value: 'custom', label: 'Custom'},
]

interface FastControlsProps {
    submode: FastMode
    onSubmodeChange: (m: FastMode) => void
    strategy: string
    onStrategyChange: (id: string) => void
    /** True while Fast Mode is the live sub-mode — locks target/strategy. */
    active: boolean
    fastStatus?: fastmode.Status
}

/**
 * The Fast Mode configuration surface shown beneath the hub: the target (what
 * to unblock) and the bypass strategy (how). It owns no start/stop control —
 * that lives in the shared primary action — and no status header, since the hub
 * already states what's running.
 */
export function FastControls({submode, onSubmodeChange, strategy, onStrategyChange, active, fastStatus}: FastControlsProps) {
    return (
        <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
                <span className="text-sm font-medium text-text">Target</span>
                <SegmentedControl
                    aria-label="Fast Mode target"
                    value={submode}
                    onValueChange={onSubmodeChange}
                    options={SUBMODE_OPTIONS}
                    disabled={active}
                />
                <p className="text-xs text-text-muted">What to unblock — all HTTPS, just Discord, or your own list.</p>
            </div>

            {submode === 'custom' && <CustomDomainEditor />}

            <StrategySelector value={strategy} onChange={onStrategyChange} disabled={active} />

            {active && fastStatus && (
                <dl className="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-border pt-4 text-sm">
                    <dt className="text-text-muted">Running for</dt>
                    <dd className="text-text">{formatDurationSince(fastStatus.since as unknown as string)}</dd>
                    <dt className="text-text-muted">Encrypted DNS</dt>
                    <dd className="text-text">{fastStatus.dnsApplied ? 'Applied' : 'Not applied'}</dd>
                    {fastStatus.restarts > 0 && (
                        <>
                            <dt className="text-text-muted">Restarts</dt>
                            <dd className="text-text">{fastStatus.restarts}</dd>
                        </>
                    )}
                </dl>
            )}
        </div>
    )
}
