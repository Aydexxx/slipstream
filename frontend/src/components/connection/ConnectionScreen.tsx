import {useEffect, useState} from 'react'
import {GetCustomDomains} from '../../../wailsjs/go/app/App'
import {useStrategies} from '../../hooks/useStrategies'
import {normalizeError} from '../../lib/errors'
import type {FastMode} from '../../state/AppStateContext'
import {useAppState} from '../../state/useAppState'
import {FastControls} from '../fast/FastControls'
import {StatusHub} from '../status/StatusHub'
import {Button} from '../ui/Button'
import {Card} from '../ui/Card'
import {useToast} from '../ui/Toast'

interface PrimaryAction {
    label: string
    variant: 'primary' | 'secondary'
    onClick: () => void
    disabled?: boolean
}

/**
 * The main connection screen: the status hub (ground truth), one primary
 * action whose meaning follows the live state, and the Fast Mode
 * configuration. It issues requests but never predicts their outcome — the hub
 * only ever moves when a real status event lands.
 */
export function ConnectionScreen() {
    const {status, action, requestFastMode, requestIdle} = useAppState()
    const toast = useToast()
    const {defaultId} = useStrategies()

    const [fastSubmode, setFastSubmode] = useState<FastMode>('full')
    const [fastStrategy, setFastStrategy] = useState('')
    const [starting, setStarting] = useState(false)

    // Seed the strategy picker once from the persisted choice (falling back to
    // the backend default) as soon as either resolves.
    useEffect(() => {
        if (fastStrategy) return
        const seed = status?.lastFastStrategy || defaultId
        if (seed) setFastStrategy(seed)
    }, [status?.lastFastStrategy, defaultId, fastStrategy])

    const fastActive = status?.subMode === 'fast'
    const busy = starting || action.pending || status?.transitioning === true

    async function startFast() {
        setStarting(true)
        try {
            // Custom mode needs the freshest persisted list (the editor owns its
            // own save flow), so fetch it at click time rather than from cache.
            const domains = fastSubmode === 'custom' ? await GetCustomDomains() : []
            await requestFastMode(fastSubmode, fastStrategy, domains)
        } catch (err) {
            toast.push(normalizeError(err), 'error')
        } finally {
            setStarting(false)
        }
    }

    const primary: PrimaryAction = fastActive
        ? {label: 'Turn Off', variant: 'secondary', onClick: () => void requestIdle()}
        : {label: 'Turn On', variant: 'primary', onClick: () => void startFast()}

    return (
        <div className="mx-auto flex w-full max-w-md flex-col items-center gap-8 px-6 pt-8 pb-4">
            <StatusHub status={status} />

            <Button
                variant={primary.variant}
                size="lg"
                className="w-full max-w-xs"
                loading={busy}
                // Until the first real status lands we don't know the true
                // state, so we don't offer an action that assumes one.
                disabled={primary.disabled || !status}
                onClick={primary.onClick}
            >
                {primary.label}
            </Button>

            <Card className="w-full animate-slide-up">
                <FastControls
                    submode={fastSubmode}
                    onSubmodeChange={setFastSubmode}
                    strategy={fastStrategy}
                    onStrategyChange={setFastStrategy}
                    active={fastActive}
                    fastStatus={status?.fastStatus}
                />
            </Card>
        </div>
    )
}
