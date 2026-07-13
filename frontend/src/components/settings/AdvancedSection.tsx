import {useState} from 'react'
import {ResetAndQuit, Uninstall} from '../../../wailsjs/go/app/App'
import {normalizeError} from '../../lib/errors'
import {Button} from '../ui/Button'
import {ConfirmDialog} from '../ui/ConfirmDialog'

// Reset & Quit and Uninstall both end the app; ResetAndQuit resolves after the
// window closes (or not at all), so we don't await a result — we just fire it
// and let the backend take over. Uninstall can fail to *launch* the helper, in
// which case we surface that error and stay open.
export function AdvancedSection() {
    const [confirmingUninstall, setConfirmingUninstall] = useState(false)
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState<string | null>(null)

    function handleReset() {
        setBusy(true)
        setError(null)
        // Fire-and-forget: the app tears down networking and quits.
        ResetAndQuit().catch((err) => {
            setBusy(false)
            setError(normalizeError(err))
        })
    }

    function handleUninstall() {
        setBusy(true)
        setError(null)
        Uninstall().catch((err) => {
            setBusy(false)
            setConfirmingUninstall(false)
            setError(normalizeError(err))
        })
    }

    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-3">
                <div>
                    <p className="text-sm font-medium text-text">Reset &amp; Quit</p>
                    <p className="text-xs text-text-secondary">
                        Restore your original DNS, routes, and firewall, then close Slipstream. Nothing is deleted.
                    </p>
                </div>
                <Button size="sm" variant="secondary" onClick={handleReset} disabled={busy}>
                    Reset &amp; Quit
                </Button>
            </div>

            <div className="flex items-center justify-between gap-3 rounded-md border border-danger/30 bg-danger/5 p-3">
                <div>
                    <p className="text-sm font-medium text-text">Uninstall Slipstream</p>
                    <p className="text-xs text-text-secondary">
                        Restores networking and permanently removes everything: settings, your imported config, engine
                        files, the startup entry, shortcuts, and the app itself.
                    </p>
                </div>
                <Button size="sm" variant="danger" onClick={() => setConfirmingUninstall(true)} disabled={busy}>
                    Uninstall
                </Button>
            </div>

            {error && <p className="text-xs text-danger">{error}</p>}

            <ConfirmDialog
                open={confirmingUninstall}
                onOpenChange={setConfirmingUninstall}
                tone="danger"
                title="Uninstall Slipstream?"
                description="This can't be undone. Slipstream will restore your networking, then permanently remove all of its files, settings, your imported config, and the app itself."
                confirmLabel="Remove everything"
                loading={busy}
                onConfirm={handleUninstall}
            />
        </div>
    )
}
