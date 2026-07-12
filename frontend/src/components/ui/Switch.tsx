import * as RadixSwitch from '@radix-ui/react-switch'
import {cn} from '../../lib/cn'

interface SwitchProps {
    checked: boolean
    onCheckedChange: (checked: boolean) => void
    disabled?: boolean
    'aria-label': string
}

export function Switch({checked, onCheckedChange, disabled, ...props}: SwitchProps) {
    return (
        <RadixSwitch.Root
            checked={checked}
            onCheckedChange={onCheckedChange}
            disabled={disabled}
            className={cn(
                'relative h-6 w-10 shrink-0 rounded-full border border-border-strong bg-surface-2 outline-none transition-colors',
                'data-[state=checked]:border-accent data-[state=checked]:bg-accent',
                'disabled:cursor-not-allowed disabled:opacity-50',
                'focus-visible:ring-2 focus-visible:ring-accent/50',
            )}
            {...props}
        >
            <RadixSwitch.Thumb
                className={cn(
                    'block size-4 translate-x-0.5 rounded-full bg-white shadow-sm transition-transform',
                    'data-[state=checked]:translate-x-[18px]',
                )}
            />
        </RadixSwitch.Root>
    )
}
