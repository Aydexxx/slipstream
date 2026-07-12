import {useEffect, useRef} from 'react'
import {InitializeNotifications, IsNotificationAvailable, SendNotification} from '../../wailsjs/runtime/runtime'
import {useAppState} from '../state/useAppState'

interface EdgeState {
    error: boolean
    blocked: boolean
}

/**
 * Fires OS-native notifications for the moments that matter (tunnel
 * dropped/errored, kill switch engaged while not actually connected) by
 * watching the same status the rest of the UI already trusts. Works even
 * while the window is hidden in the tray — HideWindowOnClose keeps this
 * component's effects running, it just isn't rendered.
 */
export function useTrayNotifications() {
    const {status} = useAppState()
    const readyRef = useRef(false)
    const prevRef = useRef<EdgeState | null>(null)

    useEffect(() => {
        let cancelled = false
        IsNotificationAvailable()
            .then((available) => (available ? InitializeNotifications() : undefined))
            .then(() => {
                if (!cancelled) readyRef.current = true
            })
            .catch(() => {
                // Notifications unavailable on this platform/setup — skip silently.
            })
        return () => {
            cancelled = true
        }
    }, [])

    useEffect(() => {
        if (!status) return

        const isError = status.state === 'error'
        const isBlocked = status.killSwitchArmed === true && status.state !== 'private-active' && !isError

        const prev = prevRef.current
        if (readyRef.current && prev) {
            if (isError && !prev.error) {
                SendNotification({
                    id: 'slipstream-error',
                    title: 'Slipstream',
                    body: status.error || 'Something went wrong.',
                }).catch(() => {})
            } else if (isBlocked && !prev.blocked) {
                SendNotification({
                    id: 'slipstream-kill-switch',
                    title: 'Slipstream',
                    body: 'Kill switch engaged — traffic is blocked until the tunnel reconnects.',
                }).catch(() => {})
            }
        }

        prevRef.current = {error: isError, blocked: isBlocked}
    }, [status])
}
