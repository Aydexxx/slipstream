import {forwardRef, type ButtonHTMLAttributes} from 'react'
import {Loader2} from 'lucide-react'
import {cn} from '../../lib/cn'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: Variant
    size?: Size
    loading?: boolean
}

const variantClasses: Record<Variant, string> = {
    primary: 'bg-accent text-accent-foreground hover:bg-accent-hover',
    secondary: 'bg-surface text-text border border-border hover:border-border-strong',
    ghost: 'bg-transparent text-text-secondary hover:bg-surface-2 hover:text-text',
    danger: 'bg-danger text-white hover:brightness-110',
}

const sizeClasses: Record<Size, string> = {
    sm: 'h-8 px-3 text-sm gap-1.5',
    md: 'h-10 px-4 text-sm gap-2',
    lg: 'h-12 px-6 text-base gap-2',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
    {variant = 'primary', size = 'md', loading = false, disabled, className, children, ...props},
    ref,
) {
    return (
        <button
            ref={ref}
            disabled={disabled || loading}
            className={cn(
                'inline-flex items-center justify-center rounded-md font-medium shadow-sm',
                'disabled:cursor-not-allowed disabled:opacity-50',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:ring-offset-2 focus-visible:ring-offset-bg',
                variantClasses[variant],
                sizeClasses[size],
                className,
            )}
            {...props}
        >
            {loading && <Loader2 className="size-4 animate-spin" aria-hidden />}
            {children}
        </button>
    )
})
