import {useCallback, useEffect, useState} from 'react'
import {GetCustomDomains, SaveCustomDomains} from '../../wailsjs/go/app/App'
import {normalizeError} from '../lib/errors'

export function useCustomDomains() {
    const [domains, setDomains] = useState<string[]>([])
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        let cancelled = false
        GetCustomDomains()
            .then((d) => {
                if (!cancelled) setDomains(d ?? [])
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

    const save = useCallback(async (next: string[]) => {
        setSaving(true)
        setError(null)
        try {
            await SaveCustomDomains(next)
            setDomains(next)
        } catch (err) {
            setError(normalizeError(err))
            throw err
        } finally {
            setSaving(false)
        }
    }, [])

    return {domains, loading, saving, error, save}
}
