import {forwardRef, type InputHTMLAttributes} from 'react'
import {cn} from '../../lib/cn'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(
    {className, ...props},
    ref,
) {
    return (
        <input
            ref={ref}
            className={cn(
                'h-9 w-full rounded-md border border-border bg-surface-2 px-3 text-sm text-text placeholder:text-text-muted',
                'focus-visible:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/30',
                className,
            )}
            {...props}
        />
    )
})
