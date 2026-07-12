import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {TooltipProvider} from './components/ui/Tooltip'
import {ToastProvider} from './components/ui/Toast'
import {AppStateProvider} from './state/AppStateContext'
import {ThemeProvider} from './theme/ThemeContext'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ThemeProvider>
            <ToastProvider>
                <TooltipProvider>
                    <AppStateProvider>
                        <App/>
                    </AppStateProvider>
                </TooltipProvider>
            </ToastProvider>
        </ThemeProvider>
    </React.StrictMode>
)
