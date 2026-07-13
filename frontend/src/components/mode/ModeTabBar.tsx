import {ShieldCheck, Zap} from 'lucide-react'
import type {ReactNode} from 'react'
import {cn} from '../../lib/cn'
import {fastPresentation, privatePresentation} from '../../lib/statusPresentation'
import type {ModeTab} from '../../lib/types'
import type {statemachine} from '../../../wailsjs/go/models'
import {StatusDot} from '../ui/StatusDot'

interface TabDef {
    value: ModeTab
    label: string
    icon: ReactNode
}

const TABS: TabDef[] = [
    {value: 'fast', label: 'Fast', icon: <Zap className="size-5" aria-hidden />},
    {value: 'private', label: 'Private', icon: <ShieldCheck className="size-5" aria-hidden />},
]

/**
 * Bottom navigation between the two protection modes. This is *navigation
 * only* — which screen you're configuring — while each tab's live status dot
 * reflects ground truth from the backend, independent of which tab is
 * selected. Turning a mode on/off is the hub's primary action, not this bar.
 */
export function ModeTabBar({
    value,
    onChange,
    status,
}: {
    value: ModeTab
    onChange: (tab: ModeTab) => void
    status: statemachine.Status | null
}) {
    const dots = {
        fast: fastPresentation(status),
        private: privatePresentation(status),
    }

    return (
        <nav
            aria-label="Mode"
            className="mx-auto flex w-full max-w-md items-stretch gap-1 rounded-xl border border-border bg-surface/80 p-1 shadow-sm backdrop-blur"
        >
            {TABS.map((tab) => {
                const selected = value === tab.value
                const dot = dots[tab.value]
                return (
                    <button
                        key={tab.value}
                        type="button"
                        aria-current={selected ? 'page' : undefined}
                        onClick={() => onChange(tab.value)}
                        className={cn(
                            'relative flex flex-1 items-center justify-center gap-2 rounded-lg py-2.5 text-sm font-medium outline-none',
                            'focus-visible:ring-2 focus-visible:ring-accent/50',
                            selected
                                ? 'bg-surface-2 text-text shadow-sm'
                                : 'text-text-secondary hover:text-text',
                        )}
                    >
                        {tab.icon}
                        <span>{tab.label}</span>
                        {dot.tone !== 'neutral' && (
                            <span className="absolute top-2 right-3">
                                <StatusDot tone={dot.tone} pulse={dot.pulse} />
                            </span>
                        )}
                    </button>
                )
            })}
        </nav>
    )
}
