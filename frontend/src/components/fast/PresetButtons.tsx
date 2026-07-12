import {usePresets} from '../../hooks/usePresets'
import {Button} from '../ui/Button'
import {Spinner} from '../ui/Spinner'

export function PresetButtons({onAdd}: {onAdd: (domains: string[]) => void}) {
    const {presets, loading} = usePresets()
    const entries = Object.entries(presets)

    if (loading) {
        return (
            <div className="flex items-center gap-2 py-1">
                <Spinner />
            </div>
        )
    }
    // Loaded with nothing to show (no bundled presets) - a real empty state,
    // not a loading gap, so rendering nothing here is correct.
    if (entries.length === 0) return null

    return (
        <div className="flex flex-wrap gap-2">
            {entries.map(([name, domains]) => (
                <Button key={name} size="sm" variant="secondary" onClick={() => onAdd(domains)}>
                    + {name}
                </Button>
            ))}
        </div>
    )
}
