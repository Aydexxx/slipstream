import {useState} from 'react'
import {usePrivateConfig} from '../../hooks/usePrivateConfig'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
import {Spinner} from '../ui/Spinner'
import {Textarea} from '../ui/Textarea'

export function ImportConfigForm() {
    const {status} = useAppState()
    const {hasConfig, summary, loading, busy, error, importConfig, deleteConfig} = usePrivateConfig()
    const [raw, setRaw] = useState('')
    const isPrivateActive = status?.subMode === 'private'

    async function handleImport() {
        try {
            await importConfig(raw)
            setRaw('')
        } catch {
            // error is surfaced via the hook's error state below
        }
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center py-4">
                <Spinner />
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-3">
            {hasConfig && summary ? (
                <div className="flex items-center justify-between gap-3 rounded-md border border-border bg-surface-2 px-3 py-2">
                    <div className="text-sm">
                        <p className="text-text">{summary.endpointHost}</p>
                        <p className="text-xs text-text-muted">
                            {summary.fullTunnel ? 'Full tunnel' : 'Split tunnel'} ·{' '}
                            {summary.obfuscated ? 'Obfuscated' : 'Standard'}
                        </p>
                    </div>
                    <Button size="sm" variant="danger" onClick={() => deleteConfig()} loading={busy} disabled={isPrivateActive}>
                        Remove
                    </Button>
                </div>
            ) : (
                <p className="text-sm text-text-secondary">No AmneziaWG config imported yet.</p>
            )}

            <Textarea
                rows={6}
                value={raw}
                onChange={(e) => setRaw(e.target.value)}
                placeholder="Paste your AmneziaWG config here"
            />
            <div className="flex items-center justify-between gap-3">
                {isPrivateActive && hasConfig ? (
                    <p className="text-xs text-text-muted">Disconnect Private Mode before removing its config.</p>
                ) : (
                    <span />
                )}
                <Button size="sm" onClick={handleImport} loading={busy} disabled={!raw.trim()}>
                    Import config
                </Button>
            </div>
            {error && <p className="text-xs text-danger">{error}</p>}
        </div>
    )
}
