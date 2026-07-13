import {useEffect, useState} from 'react'
import {ConnectionScreen} from './components/connection/ConnectionScreen'
import {AppShell} from './components/layout/AppShell'
import {ModeTabBar} from './components/mode/ModeTabBar'
import {SettingsDialog, type SettingsTab} from './components/settings/SettingsDialog'
import {useToast} from './components/ui/Toast'
import {useTrayNotifications} from './hooks/useTrayNotifications'
import type {ModeTab} from './lib/types'
import {useAppState} from './state/useAppState'

function App() {
    const {status, action} = useAppState()
    const toast = useToast()
    useTrayNotifications()
    const [tab, setTab] = useState<ModeTab>('fast')
    const [settings, setSettings] = useState<{open: boolean; tab: SettingsTab}>({open: false, tab: 'general'})
    const [followedInitial, setFollowedInitial] = useState(false)

    // Once, on the first real status snapshot, land on whichever mode is
    // actually running rather than always defaulting to Fast.
    useEffect(() => {
        if (!followedInitial && status) {
            if (status.subMode === 'private') setTab('private')
            else setTab('fast')
            setFollowedInitial(true)
        }
    }, [status, followedInitial])

    useEffect(() => {
        if (action.error) toast.push(action.error, 'error')
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [action.error])

    const openSettings = (settingsTab: SettingsTab = 'general') => setSettings({open: true, tab: settingsTab})

    return (
        <>
            <AppShell
                onOpenSettings={() => openSettings()}
                nav={<ModeTabBar value={tab} onChange={setTab} status={status} />}
            >
                <ConnectionScreen tab={tab} onOpenSettings={() => openSettings('network')} />
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
