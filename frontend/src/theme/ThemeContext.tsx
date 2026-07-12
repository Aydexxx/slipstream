import {createContext, useCallback, useEffect, useMemo, useState, type ReactNode} from 'react'
import {WindowSetDarkTheme, WindowSetLightTheme} from '../../wailsjs/runtime/runtime'

export type ThemePreference = 'light' | 'dark' | 'system'
export type EffectiveTheme = 'light' | 'dark'

const STORAGE_KEY = 'slipstream:theme'

function readStoredPreference(): ThemePreference {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored === 'light' || stored === 'dark' || stored === 'system' ? stored : 'system'
}

function systemPrefersDark(): boolean {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export interface ThemeContextValue {
    preference: ThemePreference
    effective: EffectiveTheme
    setPreference: (pref: ThemePreference) => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

export function ThemeProvider({children}: {children: ReactNode}) {
    const [preference, setPreferenceState] = useState<ThemePreference>(readStoredPreference)
    const [systemDark, setSystemDark] = useState(systemPrefersDark)

    useEffect(() => {
        const media = window.matchMedia('(prefers-color-scheme: dark)')
        const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches)
        media.addEventListener('change', onChange)
        return () => media.removeEventListener('change', onChange)
    }, [])

    const effective: EffectiveTheme = preference === 'system' ? (systemDark ? 'dark' : 'light') : preference

    useEffect(() => {
        const root = document.documentElement
        root.classList.toggle('dark', effective === 'dark')
        // Keep the native Windows titlebar in sync with the in-app theme.
        if (effective === 'dark') {
            WindowSetDarkTheme()
        } else {
            WindowSetLightTheme()
        }
    }, [effective])

    const setPreference = useCallback((pref: ThemePreference) => {
        localStorage.setItem(STORAGE_KEY, pref)
        setPreferenceState(pref)
    }, [])

    const value = useMemo(() => ({preference, effective, setPreference}), [preference, effective, setPreference])

    return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
