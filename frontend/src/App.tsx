import {useEffect, useState} from 'react'
import {AppShell} from './components/layout/AppShell'
import {FastModePanel} from './components/fast/FastModePanel'
import {ModeSelector} from './components/mode/ModeSelector'
import {OffPanel} from './components/mode/OffPanel'
import {PrivateModePanel} from './components/private/PrivateModePanel'
import {SettingsDialog} from './components/settings/SettingsDialog'
import {useToast} from './components/ui/Toast'
import {useTrayNotifications} from './hooks/useTrayNotifications'
import type {PanelKey} from './lib/types'
import {useAppState} from './state/useAppState'

function App() {
    const {status, action} = useAppState()
    const toast = useToast()
    useTrayNotifications()
    const [panel, setPanel] = useState<PanelKey>('off')
    const [settingsOpen, setSettingsOpen] = useState(false)
    const [followedInitial, setFollowedInitial] = useState(false)

    // Once, on the first real status snapshot, open on whichever panel
    // matches what's actually running rather than always defaulting to Off.
    useEffect(() => {
        if (!followedInitial && status) {
            if (status.subMode === 'fast') setPanel('fast')
            else if (status.subMode === 'private') setPanel('private')
            setFollowedInitial(true)
        }
    }, [status, followedInitial])

    useEffect(() => {
        if (action.error) toast.push(action.error, 'error')
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [action.error])

    return (
        <AppShell onOpenSettings={() => setSettingsOpen(true)}>
            <ModeSelector panel={panel} onPanelChange={setPanel} />
            {panel === 'off' && <OffPanel />}
            {panel === 'fast' && <FastModePanel />}
            {panel === 'private' && <PrivateModePanel onOpenSettings={() => setSettingsOpen(true)} />}
            <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
        </AppShell>
    )
}

export default App
