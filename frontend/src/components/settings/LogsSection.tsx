import {useState} from 'react'
import {OpenLogsFolder} from '../../../wailsjs/go/app/App'
import {normalizeError} from '../../lib/errors'
import {Button} from '../ui/Button'

export function LogsSection() {
    const [error, setError] = useState<string | null>(null)

    async function handleOpen() {
        try {
            await OpenLogsFolder()
            setError(null)
        } catch (err) {
            setError(normalizeError(err))
        }
    }

    return (
        <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-3">
                <div>
                    <p className="text-sm font-medium text-text">Logs</p>
                    <p className="text-xs text-text-secondary">Rotating log files for diagnosing issues.</p>
                </div>
                <Button size="sm" variant="secondary" onClick={handleOpen}>
                    Open Folder
                </Button>
            </div>
            {error && <p className="text-xs text-danger">{error}</p>}
        </div>
    )
}
