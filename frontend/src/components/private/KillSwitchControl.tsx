import {useState} from 'react'
import {useAppState} from '../../state/useAppState'
import {Button} from '../ui/Button'
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

            {confirming && (
                <div className="flex items-center justify-between gap-3 rounded-md border border-warning/30 bg-warning/10 px-3 py-2">
                    <p className="text-xs text-warning">Turn off leak protection now?</p>
                    <div className="flex shrink-0 gap-2">
                        <Button size="sm" variant="ghost" onClick={() => setConfirming(false)}>
                            Cancel
                        </Button>
                        <Button size="sm" variant="danger" onClick={handleDisarm} loading={action.pending}>
                            Confirm
                        </Button>
                    </div>
                </div>
            )}

            <Button
                variant="danger"
                size="lg"
                disabled={!armed}
                loading={action.pending}
                onClick={() => setConfirming(true)}
            >
                Restore Internet
            </Button>
        </div>
    )
}
