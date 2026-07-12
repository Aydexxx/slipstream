import * as RadixToast from '@radix-ui/react-toast'
import {CheckCircle2, XCircle} from 'lucide-react'
import {createContext, useCallback, useContext, useState, type ReactNode} from 'react'

type ToastVariant = 'success' | 'error'

interface ToastItem {
    id: number
    message: string
    variant: ToastVariant
}

interface ToastContextValue {
    push: (message: string, variant?: ToastVariant) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let nextId = 1

export function ToastProvider({children}: {children: ReactNode}) {
    const [items, setItems] = useState<ToastItem[]>([])

    const push = useCallback((message: string, variant: ToastVariant = 'error') => {
        setItems((cur) => [...cur, {id: nextId++, message, variant}])
    }, [])

    const remove = useCallback((id: number) => {
        setItems((cur) => cur.filter((t) => t.id !== id))
    }, [])

    return (
        <ToastContext.Provider value={{push}}>
            <RadixToast.Provider swipeDirection="right" duration={5000}>
                {children}
                {items.map((item) => (
                    <RadixToast.Root
                        key={item.id}
                        onOpenChange={(open) => {
                            if (!open) remove(item.id)
                        }}
                        className="animate-slide-in flex items-start gap-2 rounded-md border border-border bg-surface px-4 py-3 shadow-md"
                    >
                        {item.variant === 'success' ? (
                            <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-success" aria-hidden />
                        ) : (
                            <XCircle className="mt-0.5 size-4 shrink-0 text-danger" aria-hidden />
                        )}
                        <RadixToast.Description className="text-sm text-text">{item.message}</RadixToast.Description>
                    </RadixToast.Root>
                ))}
                <RadixToast.Viewport className="fixed right-4 bottom-4 z-50 flex w-[360px] max-w-[calc(100vw-2rem)] flex-col gap-2 outline-none" />
            </RadixToast.Provider>
        </ToastContext.Provider>
    )
}

export function useToast(): ToastContextValue {
    const ctx = useContext(ToastContext)
    if (!ctx) throw new Error('useToast must be used within a ToastProvider')
    return ctx
}
