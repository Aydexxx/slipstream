import {useEffect, useState, type ReactNode} from 'react'
import {GetVersion} from '../../../wailsjs/go/app/App'
import {useAutoStart} from '../../hooks/useAutoStart'
import {useAppState} from '../../state/useAppState'
import {Dialog} from '../ui/Dialog'
import {Switch} from '../ui/Switch'
import {AdvancedSection} from './AdvancedSection'
import {ImportConfigForm} from './ImportConfigForm'
import {LogsSection} from './LogsSection'
import {ThemeToggle} from './ThemeToggle'

function Section({title, description, children}: {title: string; description?: string; children: ReactNode}) {
    return (
        <section className="flex flex-col gap-3 border-t border-border pt-5 first:border-t-0 first:pt-0">
            <div>
                <h3 className="text-sm font-semibold text-text">{title}</h3>
                {description && <p className="text-xs text-text-secondary">{description}</p>}
            </div>
            {children}
        </section>
    )
}

export function SettingsDialog({open, onOpenChange}: {open: boolean; onOpenChange: (open: boolean) => void}) {
    const {status, action, setReconnectOnLaunch} = useAppState()
    const [version, setVersion] = useState('')
    const autoStart = useAutoStart()

    useEffect(() => {
        GetVersion().then(setVersion)
    }, [])

    return (
        <Dialog open={open} onOpenChange={onOpenChange} title="Settings" description="Slipstream preferences and connection setup.">
            <div className="flex flex-col gap-5">
                <Section title="General">
                    <div className="flex items-center justify-between gap-4">
                        <div>
                            <p className="text-sm text-text">Start with Windows</p>
                            <p className="text-xs text-text-secondary">Launch Slipstream at sign-in (a UAC prompt appears each time).</p>
                        </div>
                        <Switch
                            aria-label="Start with Windows"
                            checked={autoStart.enabled}
                            disabled={autoStart.loading || autoStart.busy}
                            onCheckedChange={(checked) => autoStart.setEnabled(checked)}
                        />
                    </div>
                    <div className="flex items-center justify-between gap-4">
                        <div>
                            <p className="text-sm text-text">Resume last mode on launch</p>
                            <p className="text-xs text-text-secondary">Automatically reconnect to your last mode when Slipstream starts.</p>
                        </div>
                        <Switch
                            aria-label="Resume last mode on launch"
                            checked={status?.reconnectOnLaunch === true}
                            disabled={action.pending}
                            onCheckedChange={(checked) => setReconnectOnLaunch(checked)}
                        />
                    </div>
                    {autoStart.error && <p className="text-xs text-danger">{autoStart.error}</p>}
                    <div>
                        <p className="mb-2 text-sm text-text">Theme</p>
                        <ThemeToggle />
                    </div>
                </Section>

                <Section title="Private Mode" description="Import the AmneziaWG config for your VPS.">
                    <ImportConfigForm />
                </Section>

                <Section title="Diagnostics">
                    <LogsSection />
                </Section>

                <Section title="Advanced">
                    <AdvancedSection />
                </Section>

                {version && <p className="text-xs text-text-muted">Slipstream v{version}</p>}
            </div>
        </Dialog>
    )
}
