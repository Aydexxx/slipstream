import {ShieldAlert} from 'lucide-react'
import {useExternalIP} from '../../hooks/useExternalIP'
import {usePrivateConfig} from '../../hooks/usePrivateConfig'
import {formatBytes} from '../../lib/format'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
import {Card} from '../ui/Card'
import {Spinner} from '../ui/Spinner'
import {StatusDot} from '../ui/StatusDot'
import {HandshakeIndicator} from './HandshakeIndicator'
import {KillSwitchControl} from './KillSwitchControl'

export function PrivateModePanel({onOpenSettings}: {onOpenSettings: () => void}) {
    const {status, action, requestPrivateMode, requestIdle} = useAppState()
    const {hasConfig, summary, loading: configLoading} = usePrivateConfig()
    const {ip, loading: ipLoading, error: ipError} = useExternalIP()

    const isActive = status?.subMode === 'private'
    const priv = status?.privateStatus
    const connected = isActive && status?.state === 'private-active'
    const connecting = isActive && status?.state === 'private-connecting'
    const errored = isActive && status?.state === 'error'
    const busy = action.pending || status?.transitioning === true

    // Before the first State() snapshot and the config-loading check have
    // both resolved, we don't yet know whether to show "Connect" or the
    // "no config" empty state - show a neutral loading placeholder instead
    // of guessing (an actionable "Connect" button could flash for an
    // instant otherwise).
    if (configLoading || !status) {
        return (
            <Card className="flex items-center justify-center py-10">
                <Spinner />
            </Card>
        )
    }

    if (!hasConfig) {
        return (
            <Card className="flex flex-col items-center gap-3 py-10 text-center">
                <ShieldAlert className="size-8 text-text-muted" aria-hidden />
                <div>
                    <h2 className="font-display text-base font-semibold text-text">No config imported</h2>
                    <p className="mt-1 text-sm text-text-secondary">
                        Import your AmneziaWG config in Settings to use Private Mode.
                    </p>
                </div>
                <Button variant="secondary" onClick={onOpenSettings}>
                    Open Settings
                </Button>
            </Card>
        )
    }

    return (
        <Card className="flex flex-col gap-5">
            <div className="flex items-center justify-between gap-4">
                <div>
                    <h2 className="font-display text-base font-semibold text-text">Private Mode</h2>
                    <p className="text-sm text-text-secondary">
                        Full tunnel through {summary?.endpointHost || 'your VPS'}, obfuscated.
                    </p>
                </div>
                {isActive && (
                    <div className="flex items-center gap-2 text-sm text-text-secondary">
                        <StatusDot tone={errored ? 'danger' : connected ? 'success' : 'warning'} pulse={connecting} />
                        {errored ? 'Error' : connected ? 'Connected' : 'Connecting…'}
                    </div>
                )}
            </div>

            {isActive && priv && (
                <div className="flex flex-col gap-3">
                    <HandshakeIndicator lastHandshake={priv.lastHandshake as unknown as string} ageSec={priv.handshakeAgeSec} />

                    {connected && (
                        <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
                            <dt className="text-text-muted">External IP</dt>
                            <dd className="flex items-center gap-2 text-text">
                                {ipLoading && !ip ? <Spinner className="size-3.5" /> : (ip ?? (ipError ? 'Unavailable' : '—'))}
                            </dd>
                            <dt className="text-text-muted">Received / Sent</dt>
                            <dd className="text-text">
                                {formatBytes(priv.rxBytes)} / {formatBytes(priv.txBytes)}
                            </dd>
                        </dl>
                    )}

                    {errored && status?.error && (
                        <p className="rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">
                            {status.error}
                        </p>
                    )}

                    <KillSwitchControl />
                </div>
            )}

            <div className="flex justify-end">
                {isActive ? (
                    <Button variant="secondary" onClick={() => requestIdle()} loading={busy}>
                        Disconnect
                    </Button>
                ) : (
                    <Button onClick={() => requestPrivateMode()} loading={busy}>
                        Connect
                    </Button>
                )}
            </div>
        </Card>
    )
}
