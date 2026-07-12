/**
 * Wails surfaces a Go `error` as a rejected promise. Depending on the Wails
 * version/platform that rejection value is usually a plain string (the
 * error's `.Error()` text) but can also arrive as an `Error` or an arbitrary
 * object — normalize all of them to a displayable string.
 */
export function normalizeError(err: unknown): string {
	if (typeof err === 'string') return err;
	if (err instanceof Error) return err.message;
	if (err && typeof err === 'object' && 'message' in err && typeof (err as { message: unknown }).message === 'string') {
		return (err as { message: string }).message;
	}
	return 'Something went wrong';
}
