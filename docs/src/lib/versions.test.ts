import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { readBuildEnv } from "./build-env.ts";
import { joinBase, parseVersions, planVersions } from "./versions.ts";

describe("planVersions", () => {
	it("serves the default branch at the root when there is no release", () => {
		assert.deepEqual(planVersions([]), [{ label: "next", path: "/", latest: true, ref: null }]);
	});

	it("ignores pre-release and malformed tags", () => {
		assert.deepEqual(planVersions(["v1.0.0-rc1", "chart-1.0.0", "1.0.0"]), [
			{ label: "next", path: "/", latest: true, ref: null },
		]);
	});

	it("keeps the last patch of each minor, latest at the root", () => {
		assert.deepEqual(planVersions(["v0.3.1", "v0.10.0", "v0.3.10", "v0.3.2", "v0.4.0-rc1", "v0.10.1"]), [
			{ label: "next", path: "/next/", latest: false, ref: null },
			{ label: "v0.10", path: "/", latest: true, ref: "v0.10.1" },
			{ label: "v0.3", path: "/v0.3/", latest: false, ref: "v0.3.10" },
		]);
	});
});

describe("parseVersions", () => {
	it("round-trips a plan", () => {
		const plan = planVersions(["v1.0.0", "v1.1.0"]);
		assert.deepEqual(parseVersions(JSON.stringify(plan)), plan);
	});

	it("rejects a malformed path", () => {
		assert.throws(() => parseVersions('[{"label":"x","path":"nope","latest":true,"ref":null}]'));
	});
});

describe("readBuildEnv", () => {
	it("is a single unversioned build when unset", () => {
		assert.deepEqual(readBuildEnv({}), { versions: [], current: null, srcDir: "./src" });
	});

	it("finds the version being built", () => {
		const versions = planVersions(["v1.0.0"]);
		const env = readBuildEnv({
			AI_REGISTRY_DOCS_VERSIONS: JSON.stringify(versions),
			AI_REGISTRY_DOCS_VERSION: "v1.0",
			AI_REGISTRY_DOCS_SRC_DIR: "/tmp/src",
		});
		assert.equal(env.current?.ref, "v1.0.0");
		assert.equal(env.srcDir, "/tmp/src");
	});

	it("fails on an unknown version", () => {
		assert.throws(() => readBuildEnv({ AI_REGISTRY_DOCS_VERSIONS: "[]", AI_REGISTRY_DOCS_VERSION: "v9.9" }));
	});
});

describe("joinBase", () => {
	it("joins the site base and a version path", () => {
		assert.equal(joinBase("/ai-registry/", "/"), "/ai-registry/");
		assert.equal(joinBase("/ai-registry/", "/next/"), "/ai-registry/next/");
	});
});
