import {forwardRef, type TextareaHTMLAttributes} from 'react'
import {cn} from '../../lib/cn'

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
    function Textarea({className, ...props}, ref) {
        return (
            <textarea
                ref={ref}
                className={cn(
                    'w-full resize-y rounded-md border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-muted',
                    'focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/30',
                    'font-mono',
                    className,
                )}
                {...props}
            />
        )
    },
)
