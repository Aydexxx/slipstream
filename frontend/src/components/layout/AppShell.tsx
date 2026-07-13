import type {ReactNode} from 'react'
import {Header} from './Header'

/**
 * App frame: a floating header, a scrollable main area, and a persistent
 * bottom navigation slot. A soft accent glow anchored to the top gives the
 * status hub something to sit against in the wide window without competing
 * with it.
 */
export function AppShell({
    onOpenSettings,
    nav,
    children,
}: {
    onOpenSettings: () => void
    nav: ReactNode
    children: ReactNode
}) {
    return (
        <div className="relative flex h-screen flex-col overflow-hidden bg-bg text-text">
            <div
                aria-hidden
                className="pointer-events-none absolute top-[-140px] left-1/2 -z-10 size-[460px] -translate-x-1/2 rounded-full bg-accent opacity-[0.10] blur-[130px]"
            />
            <Header onOpenSettings={onOpenSettings} />
            <main className="flex-1 overflow-y-auto">{children}</main>
            <footer className="shrink-0 px-6 pt-2 pb-6">{nav}</footer>
        </div>
    )
}
