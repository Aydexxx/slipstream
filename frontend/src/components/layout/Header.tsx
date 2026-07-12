import {Monitor, Moon, Settings, Sun} from 'lucide-react'
import {IconButton} from '../ui/IconButton'
import {Tooltip} from '../ui/Tooltip'
import {useTheme} from '../../theme/useTheme'

export function Header({onOpenSettings}: {onOpenSettings: () => void}) {
    const {preference, effective, setPreference} = useTheme()

    function cycleTheme() {
        setPreference(preference === 'light' ? 'dark' : preference === 'dark' ? 'system' : 'light')
    }

    const ThemeIcon = preference === 'system' ? Monitor : effective === 'dark' ? Moon : Sun

    return (
        <header className="flex items-center justify-between border-b border-border px-6 py-4">
            <span className="font-display text-lg font-semibold tracking-tight text-text">Slipstream</span>
            <div className="flex items-center gap-1">
                <Tooltip content={`Theme: ${preference}`}>
                    <IconButton aria-label="Cycle theme" onClick={cycleTheme}>
                        <ThemeIcon className="size-4" />
                    </IconButton>
                </Tooltip>
                <Tooltip content="Settings">
                    <IconButton aria-label="Open settings" onClick={onOpenSettings}>
                        <Settings className="size-4" />
                    </IconButton>
                </Tooltip>
            </div>
        </header>
    )
}
