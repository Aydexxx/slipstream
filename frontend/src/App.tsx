import {useEffect, useState} from 'react'
import {GetVersion, IsElevated, Ping} from '../wailsjs/go/app/App'

function App() {
    const [version, setVersion] = useState<string>('...')
    const [elevated, setElevated] = useState<boolean | null>(null)
    const [pingResult, setPingResult] = useState<string>('')

    useEffect(() => {
        GetVersion().then(setVersion)
        IsElevated().then(setElevated)
    }, [])

    function ping() {
        Ping().then(setPingResult)
    }

    return (
        <div id="App" className="flex min-h-screen flex-col items-center justify-center gap-4 text-white">
            <h1 className="text-3xl font-bold">Slipstream</h1>
            <p className="text-slate-300">version {version}</p>
            <p className="text-slate-300">
                elevated: {elevated === null ? '...' : elevated ? 'yes' : 'no'}
            </p>
            <button
                className="rounded bg-sky-600 px-4 py-2 font-medium hover:bg-sky-500"
                onClick={ping}
            >
                Ping Go backend
            </button>
            {pingResult && <p className="text-emerald-400">Go replied: {pingResult}</p>}
        </div>
    )
}

export default App
