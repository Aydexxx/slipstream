import {useState} from 'react'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
import {ConfirmDialog} from '../ui/ConfirmDialog'
import {Switch} from '../ui/Switch'

export function KillSwitchControl() {
    const {status, action, disarmKillSwitch} = useAppState()
    const [confirming, setConfirming] = useState(false)
    const armed = status?.killSwitchArmed === true

    async function handleDisarm() {
        setConfirming(false)
        await disarmKillSwitch()
    }

    return (
        <div className="flex flex-col gap-3 rounded-md border border-border bg-surface-2 p-4">
            <div className="flex items-center justify-between gap-4">
                <div>
                    <p className="text-sm font-medium text-text">Kill switch</p>
                    <p className="text-xs text-text-secondary">Blocks all traffic outside the tunnel if it drops.</p>
                </div>
                <Switch
                    aria-label="Kill switch armed"
                    checked={armed}
                    disabled={!armed || action.pending}
                    onCheckedChange={(checked) => {
                        if (!checked) setConfirming(true)
                    }}
                />
            </div>

            <Button
                variant="danger"
                size="lg"
                disabled={!armed}
                loading={action.pending}
                onClick={() => setConfirming(true)}
            >
                Restore Internet
            </Button>

            <ConfirmDialog
                open={confirming}
                onOpenChange={setConfirming}
                tone="warning"
                title="Turn off leak protection?"
                description="This removes the kill switch and lets traffic flow outside the tunnel immediately. Your connection may be exposed until a mode is active again."
                confirmLabel="Restore internet"
                loading={action.pending}
                onConfirm={handleDisarm}
            />
        </div>
    )
}
