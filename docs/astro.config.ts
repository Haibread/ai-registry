import { satteri } from "@astrojs/markdown-satteri";
import starlight from "@astrojs/starlight";
import { defineConfig } from "astro/config";
import starlightLinksValidator from "starlight-links-validator";
import starlightLlmsTxt from "starlight-llms-txt";
import { baseLinks } from "./src/lib/base-links.ts";
import { readBuildEnv } from "./src/lib/build-env.ts";
import { REPOSITORY, SITE, SITE_BASE } from "./src/lib/site.ts";
import { joinBase } from "./src/lib/versions.ts";

const { versions, current, srcDir } = readBuildEnv(process.env);
const base = joinBase(SITE_BASE, current?.path ?? "/");

function llmsDetails(): string | undefined {
	if (!current || versions.length < 2) return undefined;
	const others = versions
		.filter((version) => version.label !== current.label)
		.map((version) => `${version.label}: ${SITE}${joinBase(SITE_BASE, version.path)}llms.txt`)
		.join(", ");
	const covers = current.label === "next" ? "the unreleased default branch" : `release ${current.label}`;
	return `This file covers ${covers}. Other versions: ${others}.`;
}

const details = llmsDetails();

export default defineConfig({
	site: SITE,
	base,
	srcDir,
	markdown: {
		processor: satteri({ mdastPlugins: [baseLinks(base)] }),
	},
	integrations: [
		starlight({
			title: "AI Registry",
			description:
				"A self-hostable registry for MCP servers and A2A agents, with a versioned HTTP API, a public catalog and an admin console.",
			logo: { src: "../logo.svg" },
			favicon: "/favicon.svg",
			social: [{ icon: "github", label: "GitHub", href: REPOSITORY }],
			editLink: { baseUrl: `${REPOSITORY}/edit/main/docs/` },
			customCss: ["./src/styles/theme.css"],
			components: {
				Banner: "./src/components/VersionBanner.astro",
				SocialIcons: "./src/components/VersionPicker.astro",
			},
			sidebar: [
				{ label: "Guides", items: [{ autogenerate: { directory: "guides" } }] },
				{ label: "Reference", items: [{ autogenerate: { directory: "reference" } }] },
			],
			plugins: [
				starlightLinksValidator({ errorOnLocalLinks: false }),
				starlightLlmsTxt(details ? { details } : {}),
			],
		}),
	],
});
