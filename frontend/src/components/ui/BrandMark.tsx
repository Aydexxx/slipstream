import {useId} from 'react'
import {cn} from '../../lib/cn'

/**
 * The Slipstream mark: three swept, tapering flow streaks — traffic slipping
 * forward through a barrier, the middle stream breaking furthest ahead. An
 * original geometric glyph (not derived from any existing logo), reused across
 * the header, favicon, and window icon so the app's identity is coherent.
 *
 * `variant`:
 *   - "tile"  — the streaks in white on a rounded brand-gradient tile (app icon).
 *   - "glyph" — the streaks alone in `currentColor` (for tinting on any surface).
 */
export function BrandMark({
    size = 24,
    variant = 'tile',
    className,
}: {
    size?: number
    variant?: 'tile' | 'glyph'
    className?: string
}) {
    const gid = useId()

    // Three streaks sharing one slope (rise/run ≈ -0.357) so they read as a
    // single aerodynamic sweep; the middle one is longest and juts furthest
    // right as the leading edge.
    const streaks = [
        {x1: 9, y1: 15, x2: 20, y2: 11.1, opacity: 0.9},
        {x1: 10, y1: 20, x2: 24, y2: 15, opacity: 1},
        {x1: 12, y1: 24.5, x2: 18, y2: 22.4, opacity: 0.78},
    ]

    const strokeColor = variant === 'tile' ? '#ffffff' : 'currentColor'

    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 32 32"
            fill="none"
            className={className}
            role="img"
            aria-label="Slipstream"
        >
            {variant === 'tile' && (
                <>
                    <defs>
                        <linearGradient id={gid} x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
                            <stop offset="0" stopColor="var(--color-brand-1)" />
                            <stop offset="1" stopColor="var(--color-brand-2)" />
                        </linearGradient>
                    </defs>
                    <rect x="1" y="1" width="30" height="30" rx="8" fill={`url(#${gid})`} />
                </>
            )}
            {streaks.map((s, i) => (
                <line
                    key={i}
                    x1={s.x1}
                    y1={s.y1}
                    x2={s.x2}
                    y2={s.y2}
                    stroke={strokeColor}
                    strokeWidth={3.2}
                    strokeLinecap="round"
                    opacity={s.opacity}
                />
            ))}
        </svg>
    )
}
