import {Power} from 'lucide-react'
import {Card} from '../ui/Card'

export function OffPanel() {
    return (
        <Card className="flex flex-col items-center gap-2 py-10 text-center">
            <Power className="size-8 text-text-muted" aria-hidden />
            <h2 className="font-display text-base font-semibold text-text">Slipstream is off</h2>
            <p className="text-sm text-text-secondary">Pick Fast or Private above to get started.</p>
        </Card>
    )
}
