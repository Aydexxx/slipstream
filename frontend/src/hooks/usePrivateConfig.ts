import {useCallback, useEffect, useState} from 'react'
import {DeletePrivateConfig, HasPrivateConfig, ImportPrivateConfig, PrivateConfigSummary} from '../../wailsjs/go/app/App'
import {privatemode} from '../../wailsjs/go/models'
import {normalizeError} from '../lib/errors'

export function usePrivateConfig() {
    const [hasConfig, setHasConfig] = useState(false)
    const [summary, setSummary] = useState<privatemode.Summary | null>(null)
    const [loading, setLoading] = useState(true)
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const refresh = useCallback(async () => {
        setLoading(true)
        try {
            const has = await HasPrivateConfig()
            setHasConfig(has)
            setSummary(has ? await PrivateConfigSummary() : null)
        } catch (err) {
            setError(normalizeError(err))
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        refresh()
    }, [refresh])

    const importConfig = useCallback(async (raw: string) => {
        setBusy(true)
        setError(null)
        try {
            const s = await ImportPrivateConfig(raw)
            setSummary(s)
            setHasConfig(true)
        } catch (err) {
            setError(normalizeError(err))
            throw err
        } finally {
            setBusy(false)
        }
    }, [])

    const deleteConfig = useCallback(async () => {
        setBusy(true)
        setError(null)
        try {
            await DeletePrivateConfig()
            setHasConfig(false)
            setSummary(null)
        } catch (err) {
            setError(normalizeError(err))
            throw err
        } finally {
            setBusy(false)
        }
    }, [])

    return {hasConfig, summary, loading, busy, error, importConfig, deleteConfig, refresh}
}
