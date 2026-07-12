import {useEffect, useState} from 'react'
import {GetExternalIP} from '../../wailsjs/go/app/App'
import {normalizeError} from '../lib/errors'
import {useAppState} from '../state/useAppState'

const POLL_INTERVAL_MS = 15000

/**
 * Polls GetExternalIP only while Private Mode is genuinely, currently
 * connected with the kill switch armed — the same condition the Go side
 * re-verifies itself before it will even attempt the lookup. Outside that
 * window this always reports null rather than a stale/cached address.
 */
export function useExternalIP() {
    const {status} = useAppState()
    const [ip, setIp] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const active = status?.state === 'private-active' && status.killSwitchArmed === true

    useEffect(() => {
        if (!active) {
            setIp(null)
            setError(null)
            setLoading(false)
            return
        }

        let cancelled = false
        const fetchIp = async () => {
            setLoading(true)
            try {
                const result = await GetExternalIP()
                if (!cancelled) {
                    setIp(result)
                    setError(null)
                }
            } catch (err) {
                if (!cancelled) {
                    setIp(null)
                    setError(normalizeError(err))
                }
            } finally {
                if (!cancelled) setLoading(false)
            }
        }

        fetchIp()
        const interval = setInterval(fetchIp, POLL_INTERVAL_MS)
        return () => {
            cancelled = true
            clearInterval(interval)
        }
    }, [active])

    return {ip, loading, error}
}
