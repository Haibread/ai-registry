/** Swaps the current version's base for the target's, keeping the page. */
export function switchVersion(pathname: string, currentBase: string, targetBase: string): string {
	const current = currentBase.endsWith("/") ? currentBase : `${currentBase}/`;
	const target = targetBase.endsWith("/") ? targetBase : `${targetBase}/`;
	if (!pathname.startsWith(current)) return target;
	return target + pathname.slice(current.length);
}
