import {Lightbulb} from 'lucide-react'
import {useStrategies, type Strategy} from '../../hooks/useStrategies'
import {cn} from '../../lib/cn'
import {Spinner} from '../ui/Spinner'

interface StrategySelectorProps {
    /** Currently selected strategy ID. */
    value: string
    onChange: (id: string) => void
    /** Disabled while Fast Mode is active — the strategy is fixed once running. */
    disabled?: boolean
}

/**
 * Picks the DPI-bypass *strategy* — the "how" of Fast Mode — independently of
 * the target sub-mode (the "what"). Turkish DPI varies by ISP, so we offer
 * general presets plus ISP-tuned ones and let the user switch until one works.
 * Options are grouped (General / Turkish ISPs) via native <optgroup> so the ISP
 * list reads clearly. The selected preset's description and a trial-and-error
 * hint are shown below, since bypass is inherently per-line trial and error.
 */
export function StrategySelector({value, onChange, disabled}: StrategySelectorProps) {
    const {strategies, loading} = useStrategies()

    if (loading) {
        return (
            <div className="flex items-center gap-2 py-1">
                <Spinner />
                <span className="text-sm text-text-muted">Loading strategies…</span>
            </div>
        )
    }
    if (strategies.length === 0) return null

    // Preserve the backend's declared order while splitting into groups.
    const groups: {name: string; items: Strategy[]}[] = []
    for (const s of strategies) {
        let g = groups.find((x) => x.name === s.group)
        if (!g) {
            g = {name: s.group, items: []}
            groups.push(g)
        }
        g.items.push(s)
    }

    const selected = strategies.find((s) => s.id === value)

    return (
        <div className="flex flex-col gap-2">
            <div className="flex flex-col gap-1">
                <label htmlFor="fast-strategy" className="text-sm font-medium text-text">
                    Bypass strategy
                </label>
                <select
                    id="fast-strategy"
                    value={value}
                    disabled={disabled}
                    onChange={(e) => onChange(e.target.value)}
                    className={cn(
                        'h-9 w-full rounded-md border border-border bg-surface-2 px-3 text-sm text-text',
                        'focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/30',
                        'disabled:cursor-not-allowed disabled:opacity-60',
                    )}
                >
                    {groups.map((g) => (
                        <optgroup key={g.name} label={g.name}>
                            {g.items.map((s) => (
                                <option key={s.id} value={s.id}>
                                    {s.name}
                                </option>
                            ))}
                        </optgroup>
                    ))}
                </select>
            </div>

            {selected && <p className="text-sm text-text-secondary">{selected.description}</p>}

            <p className="flex items-start gap-1.5 text-xs text-text-muted">
                <Lightbulb className="mt-0.5 size-3.5 shrink-0" aria-hidden />
                <span>
                    DPI differs by ISP, so bypass is trial-and-error. If a site still won't load, pick your ISP
                    above — or try another strategy — and start again.
                </span>
            </p>
        </div>
    )
}
