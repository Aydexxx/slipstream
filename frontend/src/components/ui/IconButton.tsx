import {forwardRef, type ButtonHTMLAttributes} from 'react'
import {cn} from '../../lib/cn'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    'aria-label': string
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
    {className, children, ...props},
    ref,
) {
    return (
        <button
            ref={ref}
            className={cn(
                'inline-flex size-9 items-center justify-center rounded-md text-text-secondary',
                'hover:bg-surface-2 hover:text-text',
                'disabled:cursor-not-allowed disabled:opacity-50',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50',
                className,
            )}
            {...props}
        >
            {children}
        </button>
    )
})
