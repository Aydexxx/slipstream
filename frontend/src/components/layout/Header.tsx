import {Monitor, Moon, Settings, Sun} from 'lucide-react'
import {useTheme} from '../../theme/useTheme'
import {BrandMark} from '../ui/BrandMark'
import {IconButton} from '../ui/IconButton'
import {Tooltip} from '../ui/Tooltip'

export function Header({onOpenSettings}: {onOpenSettings: () => void}) {
    const {preference, effective, setPreference} = useTheme()

    function cycleTheme() {
        setPreference(preference === 'light' ? 'dark' : preference === 'dark' ? 'system' : 'light')
    }

    const ThemeIcon = preference === 'system' ? Monitor : effective === 'dark' ? Moon : Sun

    return (
        <header className="flex shrink-0 items-center justify-between px-6 py-4">
            <div className="flex items-center gap-2.5">
                <BrandMark size={26} />
                <span className="font-display text-lg font-semibold tracking-tight text-text">Slipstream</span>
            </div>
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
