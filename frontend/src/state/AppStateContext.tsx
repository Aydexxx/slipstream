import {createContext, useCallback, useEffect, useRef, useState, type ReactNode} from 'react'
import {RequestFastMode, RequestIdle, SetReconnectOnLaunch, State} from '../../wailsjs/go/app/App'
import {statemachine} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {normalizeError} from '../lib/errors'

export type FastMode = 'full' | 'discord' | 'custom'

interface ActionState {
    pending: boolean
    error: string | null
}

export interface AppStateValue {
    /** null until the first State() snapshot resolves. */
    status: statemachine.Status | null
    action: ActionState
    clearActionError: () => void
    requestFastMode: (mode: FastMode, strategyId: string, domains: string[]) => Promise<void>
    requestIdle: () => Promise<void>
    setReconnectOnLaunch: (enabled: boolean) => Promise<void>
}

export const AppStateContext = createContext<AppStateValue | null>(null)

/**
 * The single source of truth for backend state. `status` only ever changes
 * from a real `state:status` event (or the initial State() snapshot) — never
 * from an action being *called*, so the UI can't claim a connection exists
 * before the backend confirms it. Actions track their own pending/error UI
 * chrome locally; that's presentation, not a claim about network state.
 */
export function AppStateProvider({children}: {children: ReactNode}) {
    const [status, setStatus] = useState<statemachine.Status | null>(null)
    const [action, setAction] = useState<ActionState>({pending: false, error: null})
    const clearTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

    useEffect(() => {
        let cancelled = false
        State().then((s) => {
            if (!cancelled) setStatus(s)
        })
        const unsubscribe = EventsOn('state:status', (s: statemachine.Status) => {
            setStatus(s)
        })
        return () => {
            cancelled = true
            unsubscribe()
        }
    }, [])

    const clearActionError = useCallback(() => {
        setAction((a) => ({...a, error: null}))
    }, [])

    const runAction = useCallback(async (fn: () => Promise<void>) => {
        if (clearTimer.current) {
            clearTimeout(clearTimer.current)
            clearTimer.current = null
        }
        setAction({pending: true, error: null})
        try {
            await fn()
            setAction({pending: false, error: null})
        } catch (err) {
            const message = normalizeError(err)
            setAction({pending: false, error: message})
            // Errors are transient UI chrome (a toast-worthy moment), not part
            // of backend truth — fade them out on their own after a while.
            clearTimer.current = setTimeout(() => clearActionError(), 6000)
        }
    }, [clearActionError])

    const requestFastMode = useCallback(
        (mode: FastMode, strategyId: string, domains: string[]) =>
            runAction(() => RequestFastMode(mode, strategyId, domains)),
        [runAction],
    )
    const requestIdle = useCallback(() => runAction(() => RequestIdle()), [runAction])
    const setReconnectOnLaunch = useCallback(
        (enabled: boolean) => runAction(() => SetReconnectOnLaunch(enabled)),
        [runAction],
    )

    return (
        <AppStateContext.Provider
            value={{
                status,
                action,
                clearActionError,
                requestFastMode,
                requestIdle,
                setReconnectOnLaunch,
            }}
        >
            {children}
        </AppStateContext.Provider>
    )
}
