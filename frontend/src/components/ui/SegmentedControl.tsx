import * as RadioGroup from '@radix-ui/react-radio-group'
import type {ReactNode} from 'react'
import {cn} from '../../lib/cn'

export interface SegmentedOption<T extends string> {
    value: T
    label: string
    icon?: ReactNode
    /** Rendered after the label — used for a live status dot, for example. */
    indicator?: ReactNode
}

interface SegmentedControlProps<T extends string> {
    value: T
    onValueChange: (value: T) => void
    options: SegmentedOption<T>[]
    disabled?: boolean
    'aria-label': string
}

export function SegmentedControl<T extends string>({
    value,
    onValueChange,
    options,
    disabled,
    ...props
}: SegmentedControlProps<T>) {
    return (
        <RadioGroup.Root
            value={value}
            onValueChange={(v) => onValueChange(v as T)}
            disabled={disabled}
            className="inline-flex gap-1 rounded-lg border border-border bg-surface-2 p-1"
            {...props}
        >
            {options.map((opt) => (
                <RadioGroup.Item
                    key={opt.value}
                    value={opt.value}
                    className={cn(
                        'flex items-center justify-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium text-text-secondary outline-none',
                        'data-[state=checked]:bg-surface data-[state=checked]:text-text data-[state=checked]:shadow-sm',
                        'hover:text-text disabled:cursor-not-allowed disabled:opacity-50',
                        'focus-visible:ring-2 focus-visible:ring-accent/50',
                    )}
                >
                    {opt.icon}
                    {opt.label}
                    {opt.indicator}
                </RadioGroup.Item>
            ))}
        </RadioGroup.Root>
    )
}
