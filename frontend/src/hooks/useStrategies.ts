import {useEffect, useState} from 'react'
import {FastModeStrategies} from '../../wailsjs/go/app/App'
import type {fastmode} from '../../wailsjs/go/models'

export type Strategy = fastmode.StrategyInfo

/**
 * Loads the bundled DPI-bypass strategy presets (the ISP-aware "how" of Fast
 * Mode). The list is static for a given build, so a one-shot fetch is enough.
 * `defaultId` is the preset flagged as the default, used to preselect the
 * picker when the user has no persisted choice yet.
 */
export function useStrategies() {
    const [strategies, setStrategies] = useState<Strategy[]>([])
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        let cancelled = false
        FastModeStrategies()
            .then((s) => {
                if (!cancelled) setStrategies(s ?? [])
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [])

    const defaultId = strategies.find((s) => s.default)?.id ?? strategies[0]?.id ?? ''

    return {strategies, defaultId, loading}
}
