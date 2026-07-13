import type {ReactNode} from 'react'
import {Header} from './Header'

/**
 * App frame: a floating header over a scrollable main area. A soft accent glow
 * anchored to the top gives the status hub something to sit against in the wide
 * window without competing with it.
 */
export function AppShell({
    onOpenSettings,
    children,
}: {
    onOpenSettings: () => void
    children: ReactNode
}) {
    return (
        <div className="relative flex h-screen flex-col overflow-hidden bg-bg text-text">
            <div
                aria-hidden
                className="pointer-events-none absolute top-[-140px] left-1/2 -z-10 size-[460px] -translate-x-1/2 rounded-full bg-accent opacity-[0.10] blur-[130px]"
            />
            <Header onOpenSettings={onOpenSettings} />
            <main className="flex-1 overflow-y-auto pb-6">{children}</main>
        </div>
    )
}
