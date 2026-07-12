type ClassValue = string | number | false | null | undefined;

/** Joins truthy class names with a space. Deliberately tiny — no need for a dependency. */
export function cn(...values: ClassValue[]): string {
	return values.filter(Boolean).join(' ');
}
