import * as RadixTooltip from '@radix-ui/react-tooltip'
import type {ReactNode} from 'react'

export const TooltipProvider = RadixTooltip.Provider

export function Tooltip({content, children}: {content: string; children: ReactNode}) {
    return (
        <RadixTooltip.Root delayDuration={300}>
            <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
            <RadixTooltip.Portal>
                <RadixTooltip.Content
                    sideOffset={6}
                    className="animate-fade-in z-50 rounded-md border border-border bg-surface px-2.5 py-1.5 text-xs text-text shadow-sm"
                >
                    {content}
                    <RadixTooltip.Arrow className="fill-surface" />
                </RadixTooltip.Content>
            </RadixTooltip.Portal>
        </RadixTooltip.Root>
    )
}
