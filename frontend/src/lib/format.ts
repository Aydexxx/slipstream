// Go's zero time.Time{} serializes to this RFC3339 string — treat it as
// "not set" everywhere a `since` field might carry it.
const ZERO_TIME_PREFIX = '0001-01-01';

export function isZeroTime(iso: string | null | undefined): boolean {
	return !iso || iso.startsWith(ZERO_TIME_PREFIX);
}

/** "3s ago" / "2m ago" / "4h ago", or a dash for an unset (zero) time. */
export function formatRelativeTime(iso: string | null | undefined): string {
	if (isZeroTime(iso)) return '—';
	const then = new Date(iso as string).getTime();
	if (Number.isNaN(then)) return '—';
	const seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
	if (seconds < 5) return 'just now';
	if (seconds < 60) return `${seconds}s ago`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
}

/** "2m 14s" style duration since a timestamp, or a dash if unset. */
export function formatDurationSince(iso: string | null | undefined): string {
	if (isZeroTime(iso)) return '—';
	const then = new Date(iso as string).getTime();
	if (Number.isNaN(then)) return '—';
	let seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
	const hours = Math.floor(seconds / 3600);
	seconds -= hours * 3600;
	const minutes = Math.floor(seconds / 60);
	seconds -= minutes * 60;
	if (hours > 0) return `${hours}h ${minutes}m`;
	if (minutes > 0) return `${minutes}m ${seconds}s`;
	return `${seconds}s`;
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

export function formatBytes(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
	const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), BYTE_UNITS.length - 1);
	const value = bytes / 1024 ** exponent;
	return `${value >= 10 || exponent === 0 ? Math.round(value) : value.toFixed(1)} ${BYTE_UNITS[exponent]}`;
}
