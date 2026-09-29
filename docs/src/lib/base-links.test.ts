import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { markdownToHtml } from "satteri";
import { baseLinks, prefixBase } from "./base-links.ts";

describe("prefixBase", () => {
	it("prefixes root-absolute links only", () => {
		assert.equal(prefixBase("/guides/runbook/", "/ai-registry/"), "/ai-registry/guides/runbook/");
		assert.equal(prefixBase("https://example.com/", "/ai-registry/"), "https://example.com/");
		assert.equal(prefixBase("//cdn.example.com/x", "/ai-registry/"), "//cdn.example.com/x");
		assert.equal(prefixBase("#anchor", "/ai-registry/"), "#anchor");
		assert.equal(prefixBase("/guides/", "/"), "/guides/");
	});
});

describe("baseLinks", () => {
	it("rewrites inline and reference links", () => {
		const { html } = markdownToHtml("[a](/guides/) and [b][ref]\n\n[ref]: /reference/", {
			mdastPlugins: [baseLinks("/ai-registry/next/")],
		});
		assert.match(html, /href="\/ai-registry\/next\/guides\/"/);
		assert.match(html, /href="\/ai-registry\/next\/reference\/"/);
	});

	it("leaves external links alone", () => {
		const { html } = markdownToHtml("[x](https://modelcontextprotocol.io/)", {
			mdastPlugins: [baseLinks("/ai-registry/")],
		});
		assert.match(html, /href="https:\/\/modelcontextprotocol\.io\/"/);
	});
});
