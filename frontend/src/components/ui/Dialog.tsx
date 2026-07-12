import * as RadixDialog from '@radix-ui/react-dialog'
import {X} from 'lucide-react'
import type {ReactNode} from 'react'
import {IconButton} from './IconButton'

interface DialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    title: string
    description?: string
    children: ReactNode
}

export function Dialog({open, onOpenChange, title, description, children}: DialogProps) {
    return (
        <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
            <RadixDialog.Portal>
                <RadixDialog.Overlay className="animate-fade-in fixed inset-0 z-40 bg-black/40" />
                <RadixDialog.Content className="animate-scale-in fixed top-1/2 left-1/2 z-50 max-h-[85vh] w-[min(560px,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-lg border border-border bg-surface p-6 shadow-md focus:outline-none">
                    <div className="mb-4 flex items-start justify-between gap-4">
                        <div>
                            <RadixDialog.Title className="font-display text-lg font-semibold text-text">
                                {title}
                            </RadixDialog.Title>
                            {description && (
                                <RadixDialog.Description className="mt-1 text-sm text-text-secondary">
                                    {description}
                                </RadixDialog.Description>
                            )}
                        </div>
                        <RadixDialog.Close asChild>
                            <IconButton aria-label="Close">
                                <X className="size-4" />
                            </IconButton>
                        </RadixDialog.Close>
                    </div>
                    {children}
                </RadixDialog.Content>
            </RadixDialog.Portal>
        </RadixDialog.Root>
    )
}
