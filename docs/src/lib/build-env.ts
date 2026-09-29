import { parseVersions, type Version } from "./versions.ts";

export interface BuildEnv {
	versions: Version[];
	current: Version | null;
	srcDir: string;
}

/** Unset, the build is a single unversioned site from `./src`. */
export function readBuildEnv(env: NodeJS.ProcessEnv): BuildEnv {
	const versionsJson = env.AI_REGISTRY_DOCS_VERSIONS;
	const label = env.AI_REGISTRY_DOCS_VERSION;
	const srcDir = env.AI_REGISTRY_DOCS_SRC_DIR ?? "./src";
	if (!versionsJson || !label) return { versions: [], current: null, srcDir };

	const versions = parseVersions(versionsJson);
	const current = versions.find((version) => version.label === label);
	if (!current) {
		throw new Error(`AI_REGISTRY_DOCS_VERSION=${label} is not in AI_REGISTRY_DOCS_VERSIONS`);
	}
	return { versions, current, srcDir };
}
