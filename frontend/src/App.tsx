import {useEffect, useState} from 'react'
import {ConnectionScreen} from './components/connection/ConnectionScreen'
import {AppShell} from './components/layout/AppShell'
import {SettingsDialog, type SettingsTab} from './components/settings/SettingsDialog'
import {useToast} from './components/ui/Toast'
import {useTrayNotifications} from './hooks/useTrayNotifications'
import {useAppState} from './state/useAppState'

function App() {
    const {action} = useAppState()
    const toast = useToast()
    useTrayNotifications()
    const [settings, setSettings] = useState<{open: boolean; tab: SettingsTab}>({open: false, tab: 'general'})

    useEffect(() => {
        if (action.error) toast.push(action.error, 'error')
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [action.error])

    const openSettings = (settingsTab: SettingsTab = 'general') => setSettings({open: true, tab: settingsTab})

    return (
        <>
            <AppShell onOpenSettings={() => openSettings()}>
                <ConnectionScreen />
            </AppShell>
            <SettingsDialog
                open={settings.open}
                initialTab={settings.tab}
                onOpenChange={(open) => setSettings((s) => ({...s, open}))}
            />
        </>
    )
}

export default App
