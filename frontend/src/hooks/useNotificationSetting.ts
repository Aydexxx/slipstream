import {useCallback, useEffect, useState} from 'react'

const STORAGE_KEY = 'slipstream:notifications'

/**
 * User preference for OS desktop notifications, persisted client-side (the
 * backend has no notion of this — it always offers the capability; we simply
 * gate whether the frontend fires it). Defaults to on. Changes broadcast via a
 * storage event so every hook instance (including useTrayNotifications) stays
 * in sync within the session.
 */
function read(): boolean {
    return localStorage.getItem(STORAGE_KEY) !== 'off'
}

export function useNotificationSetting() {
    const [enabled, setEnabledState] = useState(read)

    useEffect(() => {
        const onChange = () => setEnabledState(read())
        window.addEventListener('storage', onChange)
        window.addEventListener('slipstream:notifications-changed', onChange)
        return () => {
            window.removeEventListener('storage', onChange)
            window.removeEventListener('slipstream:notifications-changed', onChange)
        }
    }, [])

    const setEnabled = useCallback((next: boolean) => {
        localStorage.setItem(STORAGE_KEY, next ? 'on' : 'off')
        setEnabledState(next)
        // Same-tab listeners don't get the native 'storage' event; nudge them.
        window.dispatchEvent(new Event('slipstream:notifications-changed'))
    }, [])

    return {enabled, setEnabled}
}
