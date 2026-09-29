import { z } from "zod";

export const versionSchema = z.object({
	label: z.string().min(1),
	path: z.string().regex(/^\/([\w.-]+\/)?$/),
	latest: z.boolean(),
	ref: z.string().nullable(),
});

export type Version = z.infer<typeof versionSchema>;

const STABLE_TAG = /^v(\d+)\.(\d+)\.(\d+)$/;

interface StableTag {
	tag: string;
	major: number;
	minor: number;
	patch: number;
}

function parseStableTag(tag: string): StableTag | null {
	const match = STABLE_TAG.exec(tag);
	if (!match) return null;
	const [, major, minor, patch] = match;
	return {
		tag,
		major: Number(major),
		minor: Number(minor),
		patch: Number(patch),
	};
}

/**
 * The latest release serves `/`, the default branch `/next/`, each older minor
 * `/vX.Y/` at its last patch. `ref: null` is the working tree.
 */
export function planVersions(tags: readonly string[]): Version[] {
	const lastPatchByMinor = new Map<string, StableTag>();
	for (const tag of tags) {
		const parsed = parseStableTag(tag);
		if (!parsed) continue;
		const key = `v${parsed.major}.${parsed.minor}`;
		const current = lastPatchByMinor.get(key);
		if (!current || parsed.patch > current.patch) lastPatchByMinor.set(key, parsed);
	}

	const releases = [...lastPatchByMinor.entries()].sort(
		([, a], [, b]) => b.major - a.major || b.minor - a.minor,
	);
	if (releases.length === 0) {
		return [{ label: "next", path: "/", latest: true, ref: null }];
	}

	return [
		{ label: "next", path: "/next/", latest: false, ref: null },
		...releases.map(([label, release], index) => ({
			label,
			path: index === 0 ? "/" : `/${label}/`,
			latest: index === 0,
			ref: release.tag,
		})),
	];
}

export function parseVersions(json: string): Version[] {
	return z.array(versionSchema).parse(JSON.parse(json));
}

export function joinBase(siteBase: string, versionPath: string): string {
	return `${siteBase.replace(/\/$/, "")}${versionPath}`;
}
