import {useState} from 'react'
import {GetCustomDomains} from '../../../wailsjs/go/app/App'
import {normalizeError} from '../../lib/errors'
import {formatDurationSince} from '../../lib/format'
import type {FastMode} from '../../state/AppStateContext'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
import {Card} from '../ui/Card'
import {SegmentedControl, type SegmentedOption} from '../ui/SegmentedControl'
import {StatusDot} from '../ui/StatusDot'
import {useToast} from '../ui/Toast'
import {CustomDomainEditor} from './CustomDomainEditor'

const SUBMODE_OPTIONS: SegmentedOption<FastMode>[] = [
    {value: 'full', label: 'Full'},
    {value: 'discord', label: 'Discord'},
    {value: 'custom', label: 'Custom'},
]

export function FastModePanel() {
    const {status, action, requestFastMode, requestIdle} = useAppState()
    const toast = useToast()
    const isActive = status?.subMode === 'fast'
    const [pendingSubMode, setPendingSubMode] = useState<FastMode>(
        (status?.fastStatus?.mode as FastMode | undefined) ?? 'full',
    )
    const [starting, setStarting] = useState(false)

    const submode = isActive ? ((status?.fastStatus?.mode as FastMode | undefined) ?? pendingSubMode) : pendingSubMode
    const showError = isActive && status?.state === 'error'
    const busy = starting || action.pending || status?.transitioning === true

    async function handleStart() {
        setStarting(true)
        try {
            // Custom mode needs the persisted domain list. Fetched fresh here
            // rather than from a cached hook, since CustomDomainEditor owns
            // its own save flow and this component must see the latest saved
            // value.
            const domains = submode === 'custom' ? await GetCustomDomains() : []
            await requestFastMode(submode, domains)
        } catch (err) {
            toast.push(normalizeError(err), 'error')
        } finally {
            setStarting(false)
        }
    }

    return (
        <Card className="flex flex-col gap-5">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="font-display text-base font-semibold text-text">Fast Mode</h2>
                    <p className="text-sm text-text-secondary">Defeats DPI without a full tunnel — no speed loss.</p>
                </div>
                {isActive && (
                    <div className="flex items-center gap-2 text-sm text-text-secondary">
                        <StatusDot tone={showError ? 'danger' : 'success'} />
                        {showError ? 'Error' : 'Active'}
                    </div>
                )}
            </div>

            <SegmentedControl
                aria-label="Fast Mode sub-mode"
                value={submode}
                onValueChange={setPendingSubMode}
                options={SUBMODE_OPTIONS}
                disabled={isActive}
            />

            {submode === 'custom' && <CustomDomainEditor />}

            {isActive && status?.fastStatus && (
                <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
                    <dt className="text-text-muted">Running for</dt>
                    <dd className="text-text">{formatDurationSince(status.fastStatus.since as unknown as string)}</dd>
                    <dt className="text-text-muted">Encrypted DNS</dt>
                    <dd className="text-text">{status.fastStatus.dnsApplied ? 'Applied' : 'Not applied'}</dd>
                    {status.fastStatus.restarts > 0 && (
                        <>
                            <dt className="text-text-muted">Restarts</dt>
                            <dd className="text-text">{status.fastStatus.restarts}</dd>
                        </>
                    )}
                </dl>
            )}

            {showError && status?.error && (
                <p className="rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">
                    {status.error}
                </p>
            )}

            <div className="flex justify-end">
                {isActive ? (
                    <Button variant="secondary" onClick={() => requestIdle()} loading={busy}>
                        Turn Off
                    </Button>
                ) : (
                    <Button onClick={handleStart} loading={busy}>
                        Start Fast Mode
                    </Button>
                )}
            </div>
        </Card>
    )
}
