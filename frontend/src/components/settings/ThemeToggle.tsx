import {Monitor, Moon, Sun} from 'lucide-react'
import type {ThemePreference} from '../../theme/ThemeContext'
import {useTheme} from '../../theme/useTheme'
import {SegmentedControl, type SegmentedOption} from '../ui/SegmentedControl'

const OPTIONS: SegmentedOption<ThemePreference>[] = [
    {value: 'light', label: 'Light', icon: <Sun className="size-4" aria-hidden />},
    {value: 'dark', label: 'Dark', icon: <Moon className="size-4" aria-hidden />},
    {value: 'system', label: 'System', icon: <Monitor className="size-4" aria-hidden />},
]

export function ThemeToggle() {
    const {preference, setPreference} = useTheme()
    return <SegmentedControl aria-label="Theme" value={preference} onValueChange={setPreference} options={OPTIONS} />
}
