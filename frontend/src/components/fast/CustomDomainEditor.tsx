import {useEffect, useState} from 'react'
import {useCustomDomains} from '../../hooks/useCustomDomains'
import {Button} from '../ui/Button'
import {Textarea} from '../ui/Textarea'
import {PresetButtons} from './PresetButtons'

function parseDomains(text: string): string[] {
    return text
        .split('\n')
        .map((d) => d.trim())
        .filter(Boolean)
}

export function CustomDomainEditor() {
    const {domains, loading, saving, error, save} = useCustomDomains()
    const [draft, setDraft] = useState('')
    const [initialized, setInitialized] = useState(false)

    useEffect(() => {
        if (!loading && !initialized) {
            setDraft(domains.join('\n'))
            setInitialized(true)
        }
    }, [loading, initialized, domains])

    const dirty = initialized && draft !== domains.join('\n')

    function addPresetDomains(list: string[]) {
        const existing = new Set(parseDomains(draft))
        for (const d of list) existing.add(d)
        setDraft(Array.from(existing).join('\n'))
    }

    async function handleSave() {
        try {
            await save(parseDomains(draft))
        } catch {
            // error is already surfaced via the hook's error state below
        }
    }

    return (
        <div className="flex flex-col gap-3">
            <PresetButtons onAdd={addPresetDomains} />
            <Textarea
                rows={6}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder={'example.com\nanother-domain.net'}
                disabled={loading}
            />
            <div className="flex items-center justify-between">
                <span className="text-xs text-text-muted">{parseDomains(draft).length} domains</span>
                <Button size="sm" onClick={handleSave} loading={saving} disabled={!dirty || loading}>
                    Save domains
                </Button>
            </div>
            {error && <p className="text-xs text-danger">{error}</p>}
        </div>
    )
}
