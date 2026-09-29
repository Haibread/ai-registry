import { execFileSync, spawnSync } from "node:child_process";
import { cpSync, mkdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { planVersions, type Version } from "../src/lib/versions.ts";

const DOCS_DIR = resolve(import.meta.dirname, "..");
const REPO_DIR = resolve(DOCS_DIR, "..");
const CONTENT_DIR = "docs/src/content/docs";
const STAGING_DIR = join(DOCS_DIR, ".versions");
const DIST_DIR = join(DOCS_DIR, "dist");

function log(event: string, fields: Record<string, string | number> = {}): void {
	const pairs = Object.entries(fields).map(([key, value]) => `${key}=${value}`);
	process.stderr.write(`${[`event=${event}`, ...pairs].join(" ")}\n`);
}

function git(...args: string[]): string {
	return execFileSync("git", args, { cwd: REPO_DIR, encoding: "utf8" });
}

function tagsWithDocs(): string[] {
	return git("tag", "--list")
		.split("\n")
		.filter(Boolean)
		.filter((tag) => {
			try {
				execFileSync("git", ["cat-file", "-e", `${tag}:${CONTENT_DIR}`], { cwd: REPO_DIR, stdio: "ignore" });
				return true;
			} catch {
				return false;
			}
		});
}

/** The current site around the tag's pages, laid out like the repository. */
function stage(version: Version, ref: string): string {
	const root = join(STAGING_DIR, version.label);
	const srcDir = join(root, "docs", "src");
	cpSync(join(DOCS_DIR, "src"), srcDir, { recursive: true });
	cpSync(join(REPO_DIR, "logo.svg"), join(root, "logo.svg"));
	rmSync(join(root, CONTENT_DIR), { recursive: true, force: true });
	execFileSync("sh", ["-c", `git archive "$REF" "$CONTENT" | tar -x -C "$ROOT"`], {
		cwd: REPO_DIR,
		env: { ...process.env, REF: ref, CONTENT: CONTENT_DIR, ROOT: root },
		stdio: ["ignore", "ignore", "inherit"],
	});
	return srcDir;
}

function build(version: Version, versions: Version[]): void {
	const srcDir = version.ref ? stage(version, version.ref) : join(DOCS_DIR, "src");
	const outDir = join(STAGING_DIR, "out", version.label);
	log("build.start", { version: version.label, ref: version.ref ?? "HEAD", path: version.path });
	const result = spawnSync(
		join(DOCS_DIR, "node_modules", ".bin", "astro"),
		["build", "--force", "--outDir", outDir],
		{
			cwd: DOCS_DIR,
			stdio: "inherit",
			env: {
				...process.env,
				AI_REGISTRY_DOCS_VERSIONS: JSON.stringify(versions),
				AI_REGISTRY_DOCS_VERSION: version.label,
				AI_REGISTRY_DOCS_SRC_DIR: srcDir,
			},
		},
	);
	if (result.status !== 0) throw new Error(`astro build failed for ${version.label}`);
	cpSync(outDir, join(DIST_DIR, version.path), { recursive: true });
	log("build.done", { version: version.label });
}

const versions = planVersions(tagsWithDocs());
log("plan", { versions: versions.map((version) => `${version.label}:${version.path}`).join(",") });
rmSync(DIST_DIR, { recursive: true, force: true });
mkdirSync(DIST_DIR, { recursive: true });
try {
	for (const version of versions) build(version, versions);
} finally {
	rmSync(STAGING_DIR, { recursive: true, force: true });
}
