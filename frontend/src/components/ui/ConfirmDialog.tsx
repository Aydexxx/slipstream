import * as RadixDialog from '@radix-ui/react-dialog'
import {AlertTriangle} from 'lucide-react'
import {cn} from '../../lib/cn'
import {Button} from './Button'

interface ConfirmDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    title: string
    description: string
    confirmLabel: string
    cancelLabel?: string
    tone?: 'danger' | 'warning'
    loading?: boolean
    onConfirm: () => void
}

/**
 * The single confirmation surface for destructive or leak-affecting actions
 * (disarming the kill switch, uninstalling). Consistent, focus-trapped, and
 * escape/overlay-dismissible so a stray click can't confirm — every such action
 * routes through here rather than ad-hoc inline confirm rows.
 */
export function ConfirmDialog({
    open,
    onOpenChange,
    title,
    description,
    confirmLabel,
    cancelLabel = 'Cancel',
    tone = 'danger',
    loading = false,
    onConfirm,
}: ConfirmDialogProps) {
    const accentRing = tone === 'danger' ? 'text-danger' : 'text-warning'

    return (
        <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
            <RadixDialog.Portal>
                <RadixDialog.Overlay className="animate-fade-in fixed inset-0 z-[60] bg-black/50 backdrop-blur-sm" />
                <RadixDialog.Content className="animate-scale-in fixed top-1/2 left-1/2 z-[70] w-[min(420px,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-surface p-6 shadow-lg focus:outline-none">
                    <div className="flex flex-col items-center gap-3 text-center">
                        <span
                            className={cn(
                                'grid size-11 place-items-center rounded-full bg-current/10',
                                accentRing,
                            )}
                        >
                            <AlertTriangle className="size-5" aria-hidden />
                        </span>
                        <RadixDialog.Title className="font-display text-lg font-semibold text-text">
                            {title}
                        </RadixDialog.Title>
                        <RadixDialog.Description className="text-sm text-text-secondary">
                            {description}
                        </RadixDialog.Description>
                    </div>
                    <div className="mt-6 flex gap-2">
                        <RadixDialog.Close asChild>
                            <Button variant="secondary" className="flex-1" disabled={loading}>
                                {cancelLabel}
                            </Button>
                        </RadixDialog.Close>
                        <Button variant="danger" className="flex-1" loading={loading} onClick={onConfirm}>
                            {confirmLabel}
                        </Button>
                    </div>
                </RadixDialog.Content>
            </RadixDialog.Portal>
        </RadixDialog.Root>
    )
}
