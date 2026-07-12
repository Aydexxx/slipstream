import type {ReactNode} from 'react'
import {Header} from './Header'

export function AppShell({onOpenSettings, children}: {onOpenSettings: () => void; children: ReactNode}) {
    return (
        <div className="min-h-screen bg-bg text-text">
            <Header onOpenSettings={onOpenSettings} />
            <main className="mx-auto flex max-w-2xl flex-col gap-6 px-6 py-10">{children}</main>
        </div>
    )
}
