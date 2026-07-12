import {useCallback, useEffect, useState} from 'react'
import {GetAutoStartEnabled, SetAutoStart} from '../../wailsjs/go/app/App'
import {normalizeError} from '../lib/errors'

export function useAutoStart() {
    const [enabled, setEnabledState] = useState(false)
    const [loading, setLoading] = useState(true)
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        let cancelled = false
        GetAutoStartEnabled()
            .then((v) => {
                if (!cancelled) setEnabledState(v)
            })
            .catch((err) => {
                if (!cancelled) setError(normalizeError(err))
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [])

    const setEnabled = useCallback(async (value: boolean) => {
        setBusy(true)
        setError(null)
        try {
            await SetAutoStart(value)
            setEnabledState(value)
        } catch (err) {
            setError(normalizeError(err))
            throw err
        } finally {
            setBusy(false)
        }
    }, [])

    return {enabled, loading, busy, error, setEnabled}
}
