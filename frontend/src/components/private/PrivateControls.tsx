import {ShieldAlert} from 'lucide-react'
import type {privatemode} from '../../../wailsjs/go/models'
import {useExternalIP} from '../../hooks/useExternalIP'
import {formatBytes} from '../../lib/format'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
import {Spinner} from '../ui/Spinner'
import {HandshakeIndicator} from './HandshakeIndicator'
import {KillSwitchControl} from './KillSwitchControl'

interface PrivateControlsProps {
    hasConfig: boolean
    summary: privatemode.Summary | null
    onOpenSettings: () => void
}

/**
 * The Private Mode detail surface shown beneath the hub: the endpoint summary,
 * live handshake/IP/throughput while connected, and the kill switch. Connect /
 * disconnect lives in the shared primary action; this owns configuration and
 * live detail only.
 */
export function PrivateControls({hasConfig, summary, onOpenSettings}: PrivateControlsProps) {
    const {status} = useAppState()
    const {ip, loading: ipLoading, error: ipError} = useExternalIP()

    const isActive = status?.subMode === 'private'
    const priv = status?.privateStatus
    const connected = isActive && status?.state === 'private-active'

    if (!hasConfig) {
        return (
            <div className="flex flex-col items-center gap-3 rounded-md border border-border bg-surface-2 px-4 py-8 text-center">
                <ShieldAlert className="size-7 text-text-muted" aria-hidden />
                <div>
                    <p className="text-sm font-medium text-text">No config imported</p>
                    <p className="mt-1 text-sm text-text-secondary">
                        Import your AmneziaWG config to use Private Mode.
                    </p>
                </div>
                <Button variant="secondary" size="sm" onClick={onOpenSettings}>
                    Open Settings
                </Button>
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-4">
            <div className="rounded-md border border-border bg-surface-2 px-4 py-3">
                <p className="text-sm text-text">{summary?.endpointHost || 'Your VPS'}</p>
                <p className="text-xs text-text-muted">
                    {summary?.fullTunnel ? 'Full tunnel' : 'Split tunnel'} ·{' '}
                    {summary?.obfuscated ? 'Obfuscated' : 'Standard'}
                </p>
            </div>

            {isActive && priv && (
                <div className="flex flex-col gap-4">
                    <HandshakeIndicator
                        lastHandshake={priv.lastHandshake as unknown as string}
                        ageSec={priv.handshakeAgeSec}
                    />

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

                    <KillSwitchControl />
                </div>
            )}
        </div>
    )
}
