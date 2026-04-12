# UI/UX Implementation Plan

> **36 accepted proposals organized into 10 dependency-ordered, independently shippable batches.**
>
> Each batch lists: proposals covered, scope estimate, whether backend/DB changes are needed, and key files affected.

---

## Dependency Graph (high-level)

```
Batch 1 (shared primitives)
  │
  ├──→ Batch 2 (listing filters & sort) ───→ Batch 5 (explore + tag cloud)
  │
  ├──→ Batch 3 (detail page restructure) ──→ Batch 6 (config generators)
  │                                     └──→ Batch 7 (publisher pages)
  │
  ├──→ Batch 4 (home page)
  │
  ├──→ Batch 8 (trust signals)
  │
  ├──→ Batch 9 (detail enrichment)
  │
  └──→ Batch 10 (admin + platform)
```

---

## Batch 1 — Shared Primitives & Foundation Components

**Proposals:** 8.2 (CopyButton), 5.2 (Loading skeletons), 5.3 (Empty state
illustrations), 5.1 (Breadcrumb navigation), 11.1 (Icon system), 10.3
(Contextual tooltips), 5.7 (Responsive detail pages)

**Scope:** M (Medium) | **Backend:** None | **DB:** None

**Rationale:** Nearly every subsequent batch depends on one or more of these
shared components. Building them first eliminates duplication and establishes
the visual vocabulary.

### Work items

1. **`CopyButton` component** (`web/src/components/ui/copy-button.tsx`)
   - Extract the copy logic from the existing `InstallCommand` into a
     standalone, reusable `CopyButton` that accepts a `value: string` prop.
     Show a check icon for 2 seconds after copy.
   - Used by: 8.3, 3.4, 2.7, 8.4, detail pages.
   - Refactor `InstallCommand` to use `CopyButton` internally.

2. **`EmptyState` component** (`web/src/components/ui/empty-state.tsx`)
   - Props: `icon` (ReactNode), `title`, `description`, `action` (optional
     ReactNode for a CTA button).
   - Replace inline empty states in `mcp/list.tsx` and `agents/list.tsx`.

3. **Loading skeletons**
   - The `Skeleton` primitive and `CardGridSkeleton` already exist.
   - Add `DetailPageSkeleton` (`web/src/components/ui/detail-page-skeleton.tsx`)
     matching the metadata grid + title + badges layout.
   - Wire skeletons into `mcp/detail.tsx`, `agents/detail.tsx`, `mcp/list.tsx`,
     `agents/list.tsx` (replace plain "Loading..." text).

4. **`Breadcrumb` component** (`web/src/components/ui/breadcrumb.tsx`)
   - Use shadcn/ui's Breadcrumb primitive.
   - Props: `segments: Array<{ label: string; href?: string }>`.
   - Format: `Home > MCP Servers > {namespace} > {slug}` (namespace is
     clickable, linking to the filtered list — ties into 7.3).
   - Add to both MCP and agent detail pages.

5. **`ResourceIcon` component** (`web/src/components/ui/resource-icon.tsx`)
   - Map resource types to lucide icons: MCP Server → `Plug`, Agent → `Bot`,
     Publisher → `Building2`, Skill → `Zap`.
   - Use consistently in navigation, cards, breadcrumbs, and search results.
   - Update `Header` nav links to use `ResourceIcon`.

6. **`TooltipInfo` component** (`web/src/components/ui/tooltip-info.tsx`)
   - Small `ⓘ` info icon wrapping shadcn/ui's `Tooltip`.
   - Props: `content: string`.
   - Create a data map at `web/src/lib/field-explanations.ts` mapping field
     names (runtime, transport types, protocol_version) to explanations.

7. **Responsive detail page fixes** (5.7)
   - Audit `mcp/detail.tsx` and `agents/detail.tsx` at 375px and 768px.
   - Change metadata `<dl>` from `grid-cols-2` to `grid-cols-1 sm:grid-cols-2`.
   - Ensure code blocks have `overflow-x-auto`.
   - Fix flex-wrap issues on badge rows and button groups.

### Files changed

- `web/src/components/ui/copy-button.tsx` (new)
- `web/src/components/ui/empty-state.tsx` (new)
- `web/src/components/ui/detail-page-skeleton.tsx` (new)
- `web/src/components/ui/breadcrumb.tsx` (new)
- `web/src/components/ui/resource-icon.tsx` (new)
- `web/src/components/ui/tooltip-info.tsx` (new)
- `web/src/components/ui/install-command.tsx` (refactor to use CopyButton)
- `web/src/lib/field-explanations.ts` (new)
- `web/src/pages/mcp/detail.tsx` (breadcrumb, skeleton, responsive)
- `web/src/pages/agents/detail.tsx` (breadcrumb, skeleton, responsive)
- `web/src/pages/mcp/list.tsx` (skeleton, empty state)
- `web/src/pages/agents/list.tsx` (skeleton, empty state)
- `web/src/components/layout/header.tsx` (resource icons)

---

## Batch 2 — Listing Page Filters, Sort & Deep Links

**Proposals:** 2.1 (Transport filter), 2.2 (Ecosystem filter — stdio only), 2.3
(Sort options), 5.4 (Deep-linkable filters), 2.5 (Skill tags on agent cards),
2.7 (Copy install command on cards), 7.3 (Namespace as clickable navigation)

**Scope:** L (Large) | **Backend:** Yes (new query parameters) | **DB:** No

**Depends on:** Batch 1 (CopyButton)

**Rationale:** These enhance the primary listing experience. Filters require
backend `sort`, `transport`, and `registry_type` query parameters.

### Backend work

1. **OpenAPI spec** — add parameters to `GET /api/v1/mcp/servers`:
   - `transport` (enum: `stdio`, `sse`, `streamable_http`) — filter on
     `latest_version.packages[].transport.type`.
   - `registry_type` (string: `npm`, `pip`, `docker`, etc.) — filter on
     `latest_version.packages[].registryType`. Only meaningful when
     transport = stdio.
   - `sort` (enum: `created_at_desc`, `updated_at_desc`, `name_asc`,
     `name_desc`) — default `created_at_desc`.

2. **OpenAPI spec** — add `sort` parameter to `GET /api/v1/agents`.

3. **Go store layer** (`server/internal/store/mcp.go`) — extend
   `ListMCPServers` query: add WHERE clauses for transport type and registry
   type (JSONB filtering), add ORDER BY switch.

4. **Go store layer** (`server/internal/store/agent.go`) — extend
   `ListAgents` with sort parameter.

5. **Go handlers** — parse new query params and pass to store.

### Frontend work

6. **Extend `FilterBar`** (`web/src/components/ui/filter-bar.tsx`):
   - Add `transport` select (All / stdio / SSE / Streamable HTTP) — MCP only.
   - Add `registryType` select — conditionally visible only when transport is
     empty or `stdio`.
   - Add `sort` select (Newest first / Recently updated / Name A-Z / Z-A).
   - All new controls sync to URL search params (deep-linkable).

7. **Skill tags on agent cards** (`web/src/components/agents/agent-card.tsx`):
   - Render the first 3 unique tags from `latest_version.skills[].tags` as
     small Badge components below the description.

8. **Copy install command on server cards** (`web/src/components/mcp/server-card.tsx`):
   - Add a CopyButton in the card footer.
   - For stdio: copy the install command. For remote: copy the endpoint URL.

9. **Clickable namespaces** (7.3):
   - In card components: wrap namespace portion as a `<Link>` to the filtered
     list page.
   - In detail pages: the breadcrumb handles this.

### Files changed

- `server/api/openapi.yaml`
- `server/internal/store/mcp.go`
- `server/internal/store/agent.go`
- `server/internal/http/handlers/` (list handlers)
- `web/src/components/ui/filter-bar.tsx`
- `web/src/components/mcp/server-card.tsx`
- `web/src/components/agents/agent-card.tsx`
- `web/src/pages/mcp/list.tsx`
- `web/src/pages/agents/list.tsx`
- `web/src/lib/schema.d.ts` (regenerated)

---

## Batch 3 — Detail Page Restructure (Tabs, Capabilities, Skills)

**Proposals:** 3.1 (Tabbed detail layout), 3.2 (Capabilities section), 4.1
(Skills as primary content), 4.5 (Surface icon_url, documentation_url,
provider, status_message), 4.4 (Input/Output modes explained)

**Scope:** M | **Backend:** Minor | **DB:** None

**Depends on:** Batch 1 (breadcrumbs, tooltips, skeletons)

**Rationale:** These restructure both detail pages for better information
hierarchy. Can run in parallel with Batch 2.

### Work items

1. **Install shadcn Tabs** if not already present.

2. **MCP detail page tabs** (`web/src/pages/mcp/detail.tsx`):
   - Refactor into: **Overview** | **Installation** | **JSON**.
   - Overview: description, metadata grid (with `TooltipInfo`), capabilities.
   - Installation: packages list with `InstallCommand` components.
   - JSON: existing `RawJsonViewer`.
   - Tabs are URL-hash-linked for shareability.

3. **Capabilities section** (3.2):
   - `MCPServerLatestVersion` does NOT currently include `capabilities` —
     update the OpenAPI schema and Go handler to include it in the latest
     version projection.
   - Render as labeled badges: `[Tools]` `[Resources]` `[Prompts]`, with
     expand-to-list for tools/resources if present.

4. **Agent detail page restructure** (`web/src/pages/agents/detail.tsx`):
   - Promote skills to the top of the page, below title/description.
   - Rich skill cards: name, description, tag pills, example prompts.
   - Surface `icon_url` as avatar next to agent name.
   - Surface `documentation_url` as a prominent link button.
   - Surface `provider` as a metadata field.
   - Surface `status_message` as an alert banner if present.
   - Verify these fields are included in the API response.

5. **Input/Output modes explained** (4.4):
   - Create mode mapping at `web/src/lib/mode-labels.ts`.
   - Replace raw badge rendering with icon + label pairs.

### Minor backend change

- Add `capabilities` field to `MCPServerLatestVersion` in `openapi.yaml`.
- Update the Go handler that builds the latest version summary to include
  capabilities.

### Files changed

- `server/api/openapi.yaml` (capabilities in MCPServerLatestVersion)
- `server/internal/http/handlers/` (capabilities projection)
- `web/src/pages/mcp/detail.tsx` (major refactor)
- `web/src/pages/agents/detail.tsx` (major refactor)
- `web/src/lib/mode-labels.ts` (new)
- `web/src/lib/schema.d.ts` (regenerated)

---

## Batch 4 — Home Page: Search, Featured, Categories

**Proposals:** 1.2 (Global search in hero), 1.1 (Featured/popular entries), 1.3
(Category/tag cloud), 10.1 ("What is MCP/A2A?" explainer), 1.6 (Stats
enrichment per resource type)

**Scope:** L | **Backend:** Yes | **DB:** Yes (1 migration)

**Depends on:** Batch 1 (skeletons, empty states)

### DB migration (`NNNNNN_featured_and_tags`)

```sql
ALTER TABLE mcp_servers ADD COLUMN featured BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE agents ADD COLUMN featured BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE mcp_servers ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agents ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_mcp_servers_featured ON mcp_servers (featured) WHERE featured = true;
CREATE INDEX idx_agents_featured ON agents (featured) WHERE featured = true;
CREATE INDEX idx_mcp_servers_tags ON mcp_servers USING GIN (tags);
CREATE INDEX idx_agents_tags ON agents USING GIN (tags);
```

### Backend work

1. **OpenAPI spec**:
   - Add `featured` and `tags` fields to `MCPServer` and `Agent` schemas.
   - Add `featured` boolean filter and `tag` string filter to list endpoints.
   - Add `featured` and `tags` to admin update request schemas.
   - Add/expose a public stats endpoint returning: total servers, total agents,
     total publishers, published-this-week per type.

2. **Go store + handler changes**: support `featured`, `tags`, `tag` filter.
   Add public stats endpoint.

### Frontend work

3. **`SearchBar` component** (`web/src/components/ui/search-bar.tsx`):
   - Prominent search input in the hero section.
   - On type (debounced 300ms): parallel queries to both list endpoints.
   - Dropdown with categorized results (MCP Servers / Agents) using
     `ResourceIcon`.
   - Click → detail page. Enter → `/explore?q=...`.

4. **Home page refactor** (`web/src/pages/home.tsx`):
   - Replace "Recent" with "Featured" using `?featured=true&limit=6`.
   - Fall back to recent if no featured entries exist.
   - Add category pill row between stats and listings.
   - Clicking a category navigates to `/mcp?tag=databases`.

5. **Protocol explainer** (`web/src/components/home/protocol-explainer.tsx`):
   - Collapsible section below the hero. Static content.

6. **Stats enrichment** (1.6):
   - 4-card row: MCP Servers (total + delta), Agents (total + delta).
   - Per resource type, not a generic "new this week".

### Files changed

- `server/migrations/NNNNNN_featured_and_tags.up.sql` (new)
- `server/migrations/NNNNNN_featured_and_tags.down.sql` (new)
- `server/api/openapi.yaml`
- `server/internal/store/mcp.go`, `agent.go`, `stats.go`
- `server/internal/http/handlers/`
- `web/src/components/ui/search-bar.tsx` (new)
- `web/src/components/home/protocol-explainer.tsx` (new)
- `web/src/lib/categories.ts` (new)
- `web/src/pages/home.tsx` (major refactor)
- `web/src/lib/schema.d.ts` (regenerated)

---

## Batch 5 — Unified Explore Page & Recently Published/Updated

**Proposals:** 7.1 (Unified explore page), 1.4 (Recently published vs recently
updated)

**Scope:** M | **Backend:** Minor | **DB:** None

**Depends on:** Batch 2 (FilterBar), Batch 4 (SearchBar)

### Work items

1. **Explore page** (`web/src/pages/explore.tsx`):
   - Route: `/explore`.
   - Type filter tabs: All / MCP Servers / Agents.
   - Reuses `FilterBar` with type-specific filters shown/hidden based on tab.
   - Fires parallel queries to both list endpoints, merges results, renders
     with `ResourceIcon` prefix.
   - Pre-fills from URL: `/explore?q=postgres`.
   - Add "Explore" to the header navigation.

2. **Home page toggle** — "New this week" vs "Recently updated":
   - Tab toggle above the entry grids on the home page.
   - "New this week": filter by `published_at` within last 7 days.
   - "Recently updated": `sort=updated_at_desc`.
   - Backend: add `sort=published_at_desc` option and optionally a
     `published_since` date filter to the list endpoints.

3. **Update router** to add `/explore` route.

4. **Update SearchBar** (from Batch 4) to navigate to `/explore?q=...` on Enter.

### Files changed

- `web/src/pages/explore.tsx` (new)
- `web/src/pages/home.tsx` (add toggle)
- `web/src/components/layout/header.tsx` (add Explore nav link)
- `server/api/openapi.yaml` (published_since filter, published_at sort)
- `server/internal/store/mcp.go`, `agent.go`
- Router configuration file

---

## Batch 6 — Config Generators (MCP + Agent)

**Proposals:** 3.4 (One-click install snippet), 8.3 (MCP config generator), 8.4
(Agent connection snippet generator), 4.3 (Authentication guide for agents)

**Scope:** M | **Backend:** None | **DB:** None

**Depends on:** Batch 1 (CopyButton), Batch 3 (tabbed layout)

**Rationale:** Highest-impact DX features. MCP hosts config is TOP priority.

### Work items

1. **MCP host config map** (`web/src/lib/mcp-host-configs.ts`):
   - Data structure mapping host names to their config format:
     - Claude Desktop: `~/Library/Application Support/Claude/claude_desktop_config.json`
     - Cursor: `.cursor/mcp.json`
     - Windsurf: `.windsurf/mcp.json`
     - VS Code + Cline: `.vscode/mcp.json`
   - Each host template: how to structure the `mcpServers` JSON block.
   - Handle env vars placeholders.

2. **`MCPConfigGenerator` component** (`web/src/components/mcp/config-generator.tsx`):
   - Host selector dropdown.
   - Reads packages from the server's API response.
   - Generates the exact JSON config block for the selected host.
   - Syntax-highlighted code block with CopyButton.
   - For remote transports: URL-based config with auth header placeholder.
   - Place in the Installation tab.

3. **`AgentSnippetGenerator` component** (`web/src/components/agents/snippet-generator.tsx`):
   - Language tabs: curl / Python / TypeScript / Go.
   - Templates parameterized with endpoint URL, auth scheme, and method.
   - Uses the A2A `tasks/send` JSON-RPC format.
   - Syntax-highlighted with CopyButton.

4. **Authentication guide** (`web/src/components/agents/auth-guide.tsx`):
   - Renders auth instructions based on declared scheme(s).
   - Bearer → show header format. OAuth2 → show flow summary. ApiKey →
     show placement.
   - Place above the snippet generator on the agent detail page.

### Files changed

- `web/src/lib/mcp-host-configs.ts` (new)
- `web/src/components/mcp/config-generator.tsx` (new)
- `web/src/components/agents/snippet-generator.tsx` (new)
- `web/src/components/agents/auth-guide.tsx` (new)
- `web/src/pages/mcp/detail.tsx` (add to Installation tab)
- `web/src/pages/agents/detail.tsx` (add auth guide + snippet generator)

---

## Batch 7 — Publisher Pages & Sidebar

**Proposals:** 7.2 (Publisher public profile pages), 3.7 (Publisher card
sidebar), 3.6 (Related servers)

**Scope:** M | **Backend:** Yes (public publisher endpoint) | **DB:** None

**Depends on:** Batch 1 (breadcrumbs), Batch 3 (detail layout)

### Backend work

1. **Public publisher endpoint** — `GET /api/v1/publishers/{slug}` (read-only):
   - Returns publisher name, slug, verified status, contact.
   - OpenAPI spec addition + Go handler.

2. **Publisher entry counts** — let the frontend query list endpoints with
   `namespace={slug}&limit=0` to get `total_count`.

### Frontend work

3. **Publisher profile page** (`web/src/pages/publishers/detail.tsx`):
   - Route: `/publishers/{slug}`.
   - Publisher info + grids of MCP servers and agents filtered by namespace.
   - Uses existing `ServerCard` and `AgentCard` components.

4. **Publisher card sidebar** (`web/src/components/shared/publisher-sidebar.tsx`):
   - Small card in the detail page showing publisher name, verified status,
     entry count, "View all entries" link.
   - Fetches publisher data by namespace slug.
   - Add to both MCP and agent detail pages.

5. **Related servers** (`web/src/components/mcp/related-servers.tsx`):
   - Bottom of MCP detail page.
   - Queries `?namespace={ns}&limit=3`, excludes current server.
   - Row of compact ServerCards. Same pattern for agents.

6. **Update router** to add `/publishers/:slug` route.

### Files changed

- `server/api/openapi.yaml` (public publisher GET endpoint)
- `server/internal/http/handlers/` (new public publisher handler)
- `server/internal/http/router.go` (new route)
- `web/src/pages/publishers/detail.tsx` (new)
- `web/src/components/shared/publisher-sidebar.tsx` (new)
- `web/src/components/mcp/related-servers.tsx` (new)
- `web/src/pages/mcp/detail.tsx` (add sidebar + related)
- `web/src/pages/agents/detail.tsx` (add sidebar)
- Router configuration

---

## Batch 8 — Trust & Quality Signals

**Proposals:** 9.1 (Verified badge on entries), 9.2 (Freshness indicator), 5.6
(Dark mode polish), 9.3 (Compatibility matrix)

**Scope:** M | **Backend:** Yes | **DB:** Yes (1 migration)

**Depends on:** Batch 1 (badges, tooltips)

### DB migration (`NNNNNN_verified_entries`)

```sql
ALTER TABLE mcp_servers ADD COLUMN verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE agents ADD COLUMN verified BOOLEAN NOT NULL DEFAULT false;
```

### Backend work

1. **OpenAPI spec**: Add `verified` field to schemas. Add admin toggle endpoint.
2. **Go store + handler**: Read/write `verified` flag.

### Frontend work

3. **`VerifiedBadge`** — add to `badge.tsx`: checkmark badge rendered on cards
   and detail pages when `verified === true`.

4. **`FreshnessIndicator`** (`web/src/components/ui/freshness-indicator.tsx`):
   - Takes `updated_at`, computes relative time.
   - Green dot: < 3 months. Yellow dot: 3-12 months. Red dot: > 12 months
     with warning text.
   - Use on cards and detail pages.

5. **Compatibility info** (`web/src/components/shared/compatibility-info.tsx`):
   - Display `protocol_version` with label + transport types.
   - Start simple; add `tested_with` field later if needed.

6. **Dark mode audit**:
   - Test every component in dark mode.
   - Focus on: badge contrast, code block backgrounds, card borders, skeleton
     animation colors.
   - Fix using Tailwind `dark:` variants.

### Files changed

- `server/migrations/NNNNNN_verified_entries.up.sql` (new)
- `server/migrations/NNNNNN_verified_entries.down.sql` (new)
- `server/api/openapi.yaml`
- `server/internal/store/mcp.go`, `agent.go`
- `web/src/components/ui/badge.tsx` (add VerifiedBadge)
- `web/src/components/ui/freshness-indicator.tsx` (new)
- `web/src/components/shared/compatibility-info.tsx` (new)
- `web/src/components/mcp/server-card.tsx`
- `web/src/components/agents/agent-card.tsx`
- Various component files (dark mode fixes)

---

## Batch 9 — Detail Enrichment & Polish

**Proposals:** 3.5 (Version history), 3.3 (README/long description), 11.5
(Sticky header with context), 11.3 (Card hover previews)

**Scope:** L | **Backend:** Yes | **DB:** Yes (1 migration)

**Depends on:** Batch 3 (tabs), Batch 6 (config generator for sticky CTA)

### DB migration (`NNNNNN_readme`)

```sql
ALTER TABLE mcp_servers ADD COLUMN readme TEXT;
ALTER TABLE agents ADD COLUMN readme TEXT;
```

### Backend work

1. **OpenAPI spec**: Add `readme` to schemas and create/update requests. Ensure
   the version list endpoint returns full version data.
2. **Go store + handler**: Read/write `readme`.

### Frontend work

3. **README rendering**:
   - Install `react-markdown` + `remark-gfm`.
   - Create `MarkdownRenderer` (`web/src/components/ui/markdown-renderer.tsx`).
   - Render in the Overview tab below the description.

4. **Version history** (`web/src/components/shared/version-history.tsx`):
   - Fetch versions endpoint.
   - Timeline list: version, publish date, status, "Latest" tag.
   - Add as a new tab: Overview | Installation | **Versions** | JSON.

5. **Sticky detail header** (`web/src/components/shared/sticky-detail-header.tsx`):
   - `IntersectionObserver` on title element.
   - Compact sticky bar: icon, name, version, status badge, CopyButton for
     install command.

6. **Card hover previews** (`web/src/components/shared/card-hover-preview.tsx`):
   - Use shadcn/ui `HoverCard`.
   - Show: capabilities/skills summary, install command with CopyButton,
     publisher info.
   - Lazy-fetch detail data on hover.

### Files changed

- `server/migrations/NNNNNN_readme.up.sql` (new)
- `server/migrations/NNNNNN_readme.down.sql` (new)
- `server/api/openapi.yaml`
- `server/internal/store/mcp.go`, `agent.go`
- `web/src/components/ui/markdown-renderer.tsx` (new)
- `web/src/components/shared/version-history.tsx` (new)
- `web/src/components/shared/sticky-detail-header.tsx` (new)
- `web/src/components/shared/card-hover-preview.tsx` (new)
- `web/src/pages/mcp/detail.tsx`
- `web/src/pages/agents/detail.tsx`
- `web/src/pages/mcp/list.tsx`
- `web/src/pages/agents/list.tsx`

---

## Batch 10 — Admin & Platform Features (v2 Foundation)

**Proposals:** 6.1 (Admin dashboard), 6.2 (Lifecycle stepper), 6.3 (Bulk
actions), 8.6 (Report issue + admin queue), 8.5 (Diff between versions), 9.4
(Health/uptime), 9.5 (Community signals), 10.2 (Getting Started guide), 10.4
(Changelog feed), 1.4 (Recently published — if not done in Batch 5)

**Scope:** XL | **Backend:** Yes (multiple endpoints, new tables) | **DB:** Yes

**Rationale:** Heaviest features and foundation for v2 platform transition.
Split into sub-batches for incremental delivery.

### Sub-batch 10a: Community Signals + Getting Started (S)

- **DB**: Add `view_count INTEGER DEFAULT 0` and `copy_count INTEGER DEFAULT 0`
  to `mcp_servers` and `agents`.
- **API**: `POST .../view` and `POST .../copy` (fire-and-forget increment).
- **Frontend**: Fire view on detail page mount. Fire copy in CopyButton.
  Display counts on cards and detail pages.
- **Getting Started page** (`web/src/pages/getting-started.tsx`): Static
  Markdown page. Link from home hero and detail pages.

### Sub-batch 10b: Admin Dashboard + Lifecycle Stepper (M)

- **API**: Extend stats endpoint with status breakdown, stale drafts list.
- **Dashboard** (`web/src/pages/admin/dashboard.tsx`): Status breakdown cards,
  recent activity timeline, action items.
- **Lifecycle stepper** (`web/src/components/admin/lifecycle-stepper.tsx`):
  Visual Draft → Published → Deprecated flow with clickable transitions.
  Replace individual status buttons on admin detail pages.

### Sub-batch 10c: Bulk Actions (M)

- **Frontend**: Row checkboxes on admin list pages. Floating action bar.
  Actions: Change status, Change visibility, Delete.
- **API**: Sequential calls to existing endpoints or new batch endpoint.

### Sub-batch 10d: Report Issue + Admin Queue (M)

- **DB**: New `reports` table with `id`, `resource_type`, `resource_id`,
  `issue_type`, `description`, `status` (pending/reviewed/dismissed),
  `created_at`, `reviewed_at`.
- **API**: `POST /api/v1/reports` (public, rate-limited),
  `GET /api/v1/reports` (admin), `PATCH /api/v1/reports/{id}` (admin).
- **Frontend**: "Report an issue" button on detail pages opens a dialog.
  Admin queue page shows pending reports.

### Sub-batch 10e: Version Diff + Changelog (M)

- **Version diff**: Auto-generated when a new version is published. Compare
  previous version's JSONB fields and store a diff summary.
- **Changelog page** (`web/src/pages/changelog.tsx`): Auto-derived from
  `published_at` timestamps. Optionally expose as RSS/Atom feed.

### Sub-batch 10f: Health/Uptime (L — may be deferred to v2 Tool Gateway)

- **Backend worker**: Periodic health check pinging remote endpoints.
- **DB**: `endpoint_health` table.
- **Frontend**: Green/yellow/red dot on cards for remote entries.

---

## Summary Table

| Batch | Proposals | Scope | Backend | DB | Depends on |
|-------|-----------|-------|---------|----|------------|
| 1 | 8.2, 5.2, 5.3, 5.1, 11.1, 10.3, 5.7 | M | No | No | — |
| 2 | 2.1, 2.2, 2.3, 5.4, 2.5, 2.7, 7.3 | L | Yes | No | Batch 1 |
| 3 | 3.1, 3.2, 4.1, 4.5, 4.4 | M | Minor | No | Batch 1 |
| 4 | 1.2, 1.1, 1.3, 10.1, 1.6 | L | Yes | Yes | Batch 1 |
| 5 | 7.1, 1.4 | M | Minor | No | Batch 2, 4 |
| 6 | 3.4, 8.3, 8.4, 4.3 | M | No | No | Batch 1, 3 |
| 7 | 7.2, 3.7, 3.6 | M | Yes | No | Batch 1, 3 |
| 8 | 9.1, 9.2, 5.6, 9.3 | M | Yes | Yes | Batch 1 |
| 9 | 3.5, 3.3, 11.5, 11.3 | L | Yes | Yes | Batch 3, 6 |
| 10 | 6.1, 6.2, 6.3, 8.6, 8.5, 9.4, 9.5, 10.2, 10.4 | XL | Yes | Yes | Most prior |

---

## Suggested Execution Timeline

```
Week 1-2:  Batch 1 (shared components — unblocks everything)
Week 2-3:  Batch 2 + Batch 3 in parallel (different page areas)
Week 3-4:  Batch 4 (home page — needs Batch 1 + migration)
Week 4-5:  Batch 5 + Batch 6 in parallel (explore page / config generators)
Week 5-6:  Batch 7 + Batch 8 in parallel (publisher pages / trust signals)
Week 6-7:  Batch 9 (detail enrichment polish)
Week 8+:   Batch 10 sub-batches, shipped incrementally
```

Batches 2 and 3 can run in parallel (listing vs detail pages).
Batches 5 and 6 can run in parallel (explore page vs config generators).
Batches 7 and 8 can run in parallel (publishers vs trust signals).
