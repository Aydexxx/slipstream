import * as RadixDialog from '@radix-ui/react-dialog'
import {Bell, SlidersHorizontal, Wrench, X, Zap} from 'lucide-react'
import {useEffect, useState, type ReactNode} from 'react'
import {GetVersion} from '../../../wailsjs/go/app/App'
import {useAutoStart} from '../../hooks/useAutoStart'
import {useNotificationSetting} from '../../hooks/useNotificationSetting'
import {cn} from '../../lib/cn'
import {useAppState} from '../../state/useAppState'
import {IconButton} from '../ui/IconButton'
import {Switch} from '../ui/Switch'
import {AdvancedSection} from './AdvancedSection'
import {ImportConfigForm} from './ImportConfigForm'
import {LogsSection} from './LogsSection'
import {ThemeToggle} from './ThemeToggle'

export type SettingsTab = 'general' | 'network' | 'notifications' | 'system'

const TABS: {value: SettingsTab; label: string; icon: ReactNode}[] = [
    {value: 'general', label: 'General', icon: <SlidersHorizontal className="size-4" aria-hidden />},
    {value: 'network', label: 'Network', icon: <Zap className="size-4" aria-hidden />},
    {value: 'notifications', label: 'Notifications', icon: <Bell className="size-4" aria-hidden />},
    {value: 'system', label: 'System', icon: <Wrench className="size-4" aria-hidden />},
]

function Row({title, description, children}: {title: string; description: string; children: ReactNode}) {
    return (
        <div className="flex items-center justify-between gap-4">
            <div>
                <p className="text-sm text-text">{title}</p>
                <p className="text-xs text-text-secondary">{description}</p>
            </div>
            {children}
        </div>
    )
}

export function SettingsDialog({
    open,
    onOpenChange,
    initialTab = 'general',
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    initialTab?: SettingsTab
}) {
    const {status, action, setReconnectOnLaunch} = useAppState()
    const autoStart = useAutoStart()
    const notifications = useNotificationSetting()
    const [version, setVersion] = useState('')
    const [tab, setTab] = useState<SettingsTab>(initialTab)

    useEffect(() => {
        GetVersion().then(setVersion)
    }, [])

    // Jump to the requested tab whenever the dialog is (re)opened.
    useEffect(() => {
        if (open) setTab(initialTab)
    }, [open, initialTab])

    return (
        <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
            <RadixDialog.Portal>
                <RadixDialog.Overlay className="animate-fade-in fixed inset-0 z-40 bg-black/40 backdrop-blur-sm" />
                <RadixDialog.Content className="animate-scale-in fixed top-1/2 left-1/2 z-50 flex h-[min(560px,calc(100vh-2rem))] w-[min(680px,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-lg border border-border bg-surface shadow-lg focus:outline-none">
                    {/* Sidebar tab rail. */}
                    <nav className="flex w-44 shrink-0 flex-col gap-1 border-r border-border bg-surface-2/50 p-3">
                        <RadixDialog.Title className="px-2 pt-1 pb-2 font-display text-base font-semibold text-text">
                            Settings
                        </RadixDialog.Title>
                        {TABS.map((t) => (
                            <button
                                key={t.value}
                                type="button"
                                onClick={() => setTab(t.value)}
                                aria-current={tab === t.value ? 'page' : undefined}
                                className={cn(
                                    'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium outline-none',
                                    'focus-visible:ring-2 focus-visible:ring-accent/50',
                                    tab === t.value
                                        ? 'bg-surface text-text shadow-sm'
                                        : 'text-text-secondary hover:bg-surface hover:text-text',
                                )}
                            >
                                {t.icon}
                                {t.label}
                            </button>
                        ))}
                        {version && <p className="mt-auto px-2 text-xs text-text-muted">Slipstream v{version}</p>}
                    </nav>

                    {/* Panel. */}
                    <div className="relative flex-1 overflow-y-auto p-6">
                        <RadixDialog.Close asChild>
                            <IconButton aria-label="Close" className="absolute top-4 right-4">
                                <X className="size-4" />
                            </IconButton>
                        </RadixDialog.Close>
                        <RadixDialog.Description className="sr-only">
                            Slipstream preferences and connection setup.
                        </RadixDialog.Description>

                        {tab === 'general' && (
                            <div className="flex flex-col gap-5">
                                <h3 className="font-display text-base font-semibold text-text">General</h3>
                                <Row
                                    title="Start with Windows"
                                    description="Launch Slipstream at sign-in (a UAC prompt appears each time)."
                                >
                                    <Switch
                                        aria-label="Start with Windows"
                                        checked={autoStart.enabled}
                                        disabled={autoStart.loading || autoStart.busy}
                                        onCheckedChange={(checked) => autoStart.setEnabled(checked)}
                                    />
                                </Row>
                                <Row
                                    title="Resume last mode on launch"
                                    description="Automatically reconnect to your last mode when Slipstream starts."
                                >
                                    <Switch
                                        aria-label="Resume last mode on launch"
                                        checked={status?.reconnectOnLaunch === true}
                                        disabled={action.pending}
                                        onCheckedChange={(checked) => setReconnectOnLaunch(checked)}
                                    />
                                </Row>
                                {autoStart.error && <p className="text-xs text-danger">{autoStart.error}</p>}
                                <div>
                                    <p className="mb-2 text-sm text-text">Theme</p>
                                    <ThemeToggle />
                                </div>
                            </div>
                        )}

                        {tab === 'network' && (
                            <div className="flex flex-col gap-4">
                                <div>
                                    <h3 className="font-display text-base font-semibold text-text">Network</h3>
                                    <p className="mt-1 text-sm text-text-secondary">
                                        Import the AmneziaWG config for your VPS. Pick your ISP-tuned bypass strategy on
                                        the Fast screen.
                                    </p>
                                </div>
                                <ImportConfigForm />
                            </div>
                        )}

                        {tab === 'notifications' && (
                            <div className="flex flex-col gap-5">
                                <h3 className="font-display text-base font-semibold text-text">Notifications</h3>
                                <Row
                                    title="Desktop notifications"
                                    description="Alert me when the tunnel drops, errors, or the kill switch blocks traffic."
                                >
                                    <Switch
                                        aria-label="Desktop notifications"
                                        checked={notifications.enabled}
                                        onCheckedChange={notifications.setEnabled}
                                    />
                                </Row>
                                <p className="text-xs text-text-muted">
                                    Slipstream only notifies for the moments that matter — never for routine activity.
                                </p>
                            </div>
                        )}

                        {tab === 'system' && (
                            <div className="flex flex-col gap-6">
                                <div>
                                    <h3 className="mb-3 font-display text-base font-semibold text-text">Diagnostics</h3>
                                    <LogsSection />
                                </div>
                                <div>
                                    <h3 className="mb-3 font-display text-base font-semibold text-text">Advanced</h3>
                                    <AdvancedSection />
                                </div>
                            </div>
                        )}
                    </div>
                </RadixDialog.Content>
            </RadixDialog.Portal>
        </RadixDialog.Root>
    )
}
