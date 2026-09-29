import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { switchVersion } from "./switch-version.ts";

describe("switchVersion", () => {
	it("keeps the page when moving to another version", () => {
		assert.equal(
			switchVersion("/ai-registry/guides/runbook/", "/ai-registry/", "/ai-registry/next/"),
			"/ai-registry/next/guides/runbook/",
		);
		assert.equal(
			switchVersion("/ai-registry/next/guides/runbook/", "/ai-registry/next/", "/ai-registry/"),
			"/ai-registry/guides/runbook/",
		);
	});

	it("accepts bases without a trailing slash", () => {
		assert.equal(switchVersion("/ai-registry/v0.3/", "/ai-registry/v0.3", "/ai-registry"), "/ai-registry/");
	});

	it("falls back to the target root outside the current base", () => {
		assert.equal(switchVersion("/elsewhere/", "/ai-registry/", "/ai-registry/next/"), "/ai-registry/next/");
	});
});
