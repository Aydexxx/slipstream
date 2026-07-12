import {useEffect, useState} from 'react'
import {FastModePresets} from '../../wailsjs/go/app/App'

export function usePresets() {
    const [presets, setPresets] = useState<Record<string, string[]>>({})
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        let cancelled = false
        FastModePresets()
            .then((p) => {
                if (!cancelled) setPresets(p ?? {})
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [])

    return {presets, loading}
}
