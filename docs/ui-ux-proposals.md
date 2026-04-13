# UI/UX Improvement Proposals

> Each proposal has a **Decision** field at the bottom. Write your verdict there:
> `accepted`, `rejected`, `deferred`, `needs-discussion`, or a free-form comment.

---

## 1. Home Page — Surface What Matters

Current state: Hero with title + 2 CTA buttons, 2 stat counter cards (MCP count,
Agent count), grid of 6 recent MCP servers, grid of 6 recent agents.

---

### 1.1 Featured / Popular entries instead of just "recent"

**Problem:** The home page shows the 6 most recently created entries. This means a
half-filled draft someone just created can push a mature, widely-used server off the
front page. "Recent" rewards activity, not quality.

**What to do:** Replace "Recent MCP Servers" / "Recent Agents" with
"Featured" or "Popular" sections. This requires either:
- A manual `featured: boolean` flag admins can toggle (simplest).
- An automatic ranking signal like view count or install/copy count (richer but
  needs tracking infrastructure).
- A hybrid: admin-curated "Staff Picks" row + an algorithmically sorted "Popular"
  row below it.

**Example — before:**
```
Recent MCP Servers
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ test-server  │ │ my-draft    │ │ broken-poc  │
│ 3 min ago    │ │ 1 hour ago  │ │ 2 hours ago │
└─────────────┘ └─────────────┘ └─────────────┘
```

**Example — after:**
```
Featured MCP Servers                          ★ Staff Picks
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ postgres-mcp │ │ github-mcp  │ │ slack-mcp   │
│ ★ Featured   │ │ ★ Featured  │ │ ★ Featured  │
│ ✓ Verified   │ │ ✓ Verified  │ │ ✓ Verified  │
└─────────────┘ └─────────────┘ └─────────────┘
```

**Scope:** Requires a `featured` column on the DB + admin toggle, or a new
`/api/v1/mcp/servers?sort=popular` query parameter with backend ranking logic.

> **Decision: Accepted**
>
> This is a good idea, and it should be replicated in the list using specific search

---

### 1.2 Global search bar in the hero

**Problem:** The hero section has a title, a subtitle, and two buttons ("Browse MCP
Servers" / "Browse Agents"). The user's #1 intent on a registry is "find something
specific." Right now they must click through to a listing page before they can type a
search query — that's one unnecessary step.

**What to do:** Add a prominent search input centered in the hero, below the subtitle.
It should search across both MCP servers and agents simultaneously and show a dropdown
with categorized results (grouped by type). Pressing Enter navigates to a results
page; clicking a result goes to its detail page.

**Example:**
```
┌──────────────────────────────────────────────────┐
│                   AI Registry                     │
│  A centralized catalog of MCP servers and agents  │
│                                                   │
│   ┌──────────────────────────────────────────┐   │
│   │ 🔍 Search servers and agents...          │   │
│   └──────────────────────────────────────────┘   │
│                                                   │
│   [Browse MCP Servers]   [Browse Agents]          │
└──────────────────────────────────────────────────┘

Dropdown on type:
┌──────────────────────────────────────────┐
│ 🔍 postgres                              │
├──────────────────────────────────────────┤
│ MCP Servers                              │
│   postgres-mcp          v1.2.0  ✓        │
│   supabase-postgres     v0.9.1           │
├──────────────────────────────────────────┤
│ Agents                                   │
│   db-migration-agent    v1.0.0  ✓        │
└──────────────────────────────────────────┘
```

**Scope:** New `SearchBar` component, a combined search API endpoint (or two parallel
queries to `/api/v1/mcp/servers?q=` and `/api/v1/agents?q=`), and a dropdown result
renderer.

> **Decision: Accepted**
>
> 

---

### 1.3 Category / tag cloud

**Problem:** MCP servers cover wildly different domains — databases, cloud APIs, file
systems, developer tools, communication platforms. Agents span task automation, code
generation, data analysis. There's no way to browse by domain on the home page. Users
must scroll through all cards or know exactly what to search for.

**What to do:** Add a horizontal row of clickable category pills between the stats
section and the listings. Clicking one navigates to the listing page pre-filtered.
Categories can be derived from tags on entries, or maintained as a curated list by
admins.

**Example:**
```
Popular categories:
[ Databases ] [ Cloud APIs ] [ DevTools ] [ File Systems ] [ Communication ]
[ Code Gen ] [ Data Analysis ] [ Security ] [ Monitoring ] [ All → ]
```

**Scope:** Requires either a `tags` field on entries (new DB column + API) or a
hardcoded category list mapped to search queries. The tag-based approach is more
scalable and also enables filtering on listing pages (see 2.2).

> **Decision: Accepted**
>
>

---

### 1.4 "Recently published" vs "Recently updated"

**Problem:** The current "Recent" sections don't distinguish between a brand-new
server that just shipped its first published version and an existing server that had a
minor metadata edit. These are very different signals to the user — one is news, the
other is maintenance.

**What to do:** Split into two sections or add a tab toggle:
- **"New this week"** — entries where `published_at` on the latest version falls
  within the last 7 days. These are genuinely new additions.
- **"Recently updated"** — entries sorted by `updated_at`. These are existing entries
  that got a new version or metadata change.

**Example:**
```
[ New this week ]   [ Recently updated ]

New this week:
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ stripe-mcp   │ │ notion-agent│ │ redis-mcp   │
│ Published    │ │ Published   │ │ Published   │
│ Apr 10, 2026 │ │ Apr 9, 2026 │ │ Apr 8, 2026 │
└─────────────┘ └─────────────┘ └─────────────┘
```

**Scope:** Minimal — uses existing `published_at` and `updated_at` fields. Just a
query parameter change and a UI toggle.

> **Decision: Accepted**
>
> _your comment here_

---

### 1.5 Publisher spotlight

**Problem:** Publishers (the organizations/individuals behind entries) are invisible on
the public UI. They only exist in the admin section. Users have no way to discover
"all servers by Anthropic" or "all agents by Datadog" from the home page.

**What to do:** Add a "Verified Publishers" row on the home page showing publisher
cards with their name, a logo/avatar (new field), and the count of published entries.
Clicking a publisher goes to their public profile (see proposal 7.2).

**Example:**
```
Verified Publishers
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│  [Logo]       │ │  [Logo]       │ │  [Logo]       │ │  [Logo]       │
│  Anthropic    │ │  Datadog      │ │  Stripe       │ │  Supabase     │
│  ✓ Verified   │ │  ✓ Verified   │ │  ✓ Verified   │ │  ✓ Verified   │
│  12 servers   │ │  3 agents     │ │  2 servers    │ │  5 servers    │
│  4 agents     │ │  1 server     │ │               │ │  2 agents     │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
```

**Scope:** Requires a public publishers API endpoint, an optional `logo_url` field on
the publisher schema, and a new `PublisherCard` component. Depends on proposal 7.2
for the link target.

> **Decision: Deferred**
>
> I still do not know if I want to keep publishers, of if I want to introduce a team/project type

---

### 1.6 Stats enrichment

**Problem:** The two stat cards only show total counts ("47 MCP Servers", "23
Agents"). This tells the user the registry exists but nothing about its health or
momentum. A registry with 47 servers and zero new ones in 3 months feels dead.

**What to do:** Enrich the stats section with more signals. Options:
- "Published this week" count (shows momentum).
- "Total publishers" (shows ecosystem breadth).
- "Total skills" (for agents — shows capability depth).
- A small sparkline or "+N this month" delta next to each count.

**Example — before:**
```
┌─────────────┐ ┌─────────────┐
│ MCP Servers  │ │ Agents      │
│     47       │ │     23      │
└─────────────┘ └─────────────┘
```

**Example — after:**
```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ MCP Servers  │ │ Agents      │ │ Publishers   │ │ New this     │
│     47       │ │     23      │ │     12       │ │ week         │
│  +5 this mo. │ │  +3 this mo.│ │  ✓ 8 verified│ │     7        │
└─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘
```

**Scope:** Requires new aggregate query endpoints or extending existing list endpoints
with count metadata (e.g., `published_since` filter + `total_count`).

> **Decision: Accepted**
>
> This should be specific to a ressource type, so the "New this week" without resource type doesn't look interesting

---

## 2. Listing Pages — Better Scanning & Filtering

Current state: Grid of cards with search input (`q`), namespace dropdown, and status
dropdown. No sorting controls. No ecosystem or transport filters.

---

### 2.1 Filter by transport type (stdio / HTTP / SSE)

**Problem:** The most fundamental decision when evaluating an MCP server is: "Is this
local (stdio) or remote (HTTP/SSE)?" This determines whether you run a binary on your
machine or connect to a hosted endpoint. Currently, transport type is shown as a small
badge buried inside each card — users must visually scan every card to find what they
want.

**What to do:** Add a filter dropdown or pill group for transport type on the MCP
server listing page. Options: `All`, `stdio` (local), `http`, `sse`,
`streamable_http`.

**Example:**
```
Transport: [ All ▾ ]  [ stdio ]  [ HTTP ]  [ SSE ]  [ Streamable HTTP ]

Search: [________________]   Namespace: [ All ▾ ]   Status: [ All ▾ ]
```

**Scope:** Requires a `transport` query parameter on the server-side
`GET /api/v1/mcp/servers` endpoint (filter on `packages->transport->type` in the
latest version's JSONB). Frontend adds the filter control and passes it as a query
param.

> **Decision: Accepted**
>
> In addition, please note that the MCP Server can only be stdio, SSE or Streamable HTTP, and not HTTP

---

### 2.2 Filter by ecosystem / registry type (npm, pip, Docker, etc.)

**Problem:** A Python developer looking for pip-installable MCP servers has no way to
filter out npm packages. The ecosystem/registry type (`registryType` in `packages[]`)
is shown as a small badge on cards but isn't filterable.

**What to do:** Add an ecosystem filter (dropdown or pill group) on the MCP listing
page. Values are derived from existing `packages[].registryType` data: `npm`, `pip`,
`docker`, `cargo`, etc.

**Example:**
```
Ecosystem: [ All ▾ ]  [ npm ]  [ pip ]  [ Docker ]  [ cargo ]  [ go ]
```

**Scope:** Similar to 2.1 — requires a `registry_type` query parameter on the backend
that filters on the JSONB `packages` array. Frontend adds the control.

> **Decision: Accepted (conditional)**
>
> Only show this filter when transport is `stdio` (local servers). For remote
> servers the ecosystem is irrelevant — users just connect to a URL.
> Implementation: hide the ecosystem filter when transport filter is set to
> SSE or Streamable HTTP.

---

### 2.3 Sort options (newest, name, recently updated)

**Problem:** Listings have a single implicit sort order (by creation date, newest
first). Users can't sort alphabetically to find a specific name, or by update date to
see what's actively maintained.

**What to do:** Add a sort dropdown with options:
- Newest first (default, current behavior)
- Recently updated
- Name A-Z
- Name Z-A

**Example:**
```
Sort by: [ Newest first ▾ ]
         ┌──────────────────┐
         │ Newest first     │
         │ Recently updated │
         │ Name A → Z       │
         │ Name Z → A       │
         └──────────────────┘
```

**Scope:** Requires a `sort` query parameter on the API (`created_at_desc`,
`updated_at_desc`, `name_asc`, `name_desc`). Frontend adds the dropdown and passes it.

> **Decision: Accepted (v1 basic, v2 expanded)**
>
> Ship basic sorts now (newest, recently updated, name A-Z/Z-A). Add
> "Popular" and "Most used" sort options in v2 when the Tool Gateway
> provides usage data for the platform transition.

---

### 2.4 List view toggle (grid vs. compact table)

**Problem:** The card grid layout (3 columns) works well for casual browsing but is
inefficient for power users who need to scan 50+ entries quickly. Each card takes
significant vertical space; a table row can show the same key info in a single line.

**What to do:** Add a toggle button (grid icon / list icon) that switches between the
current card grid and a compact table view. The table shows: Name, Namespace,
Version, Status, Transport/Ecosystem, Updated date — one row per entry.

**Example — table view:**
```
[ ⊞ Grid ]  [ ≡ Table ]

Name                 Namespace    Version  Status     Transport  Updated
─────────────────────────────────────────────────────────────────────────
postgres-mcp         anthropic    v1.2.0   Published  stdio      2 days ago
github-mcp           anthropic    v2.0.1   Published  HTTP       1 week ago
my-test-server       personal     v0.1.0   Draft      stdio      3 hours ago
deprecated-thing     legacy       v0.5.0   Deprecated SSE        6 months ago
```

**Scope:** New `ServerTable` / `AgentTable` components. Persist the user's preference
in localStorage. The API doesn't change — same data, different rendering.

> **Decision: free form**
>
> I would need to see a concrete example to know if it makes sense

---

### 2.5 Skill count / capability indicators on agent cards

**Problem:** The current agent card shows a numeric skill count badge (e.g., "3
skills") but doesn't hint at what those skills do. A user scanning 20 agent cards has
no way to tell if an agent does "code review" or "data analysis" without clicking into
each one.

**What to do:** Show the top 2-3 skill tags or skill names directly on the card,
below the description. This gives an at-a-glance sense of what the agent can do.

**Example — before:**
```
┌──────────────────────────────┐
│ Code Review Agent      v1.0  │
│ anthropic/code-review        │
│ [Published] [3 skills]       │
│                              │
│ An agent that reviews code   │
│ and suggests improvements... │
└──────────────────────────────┘
```

**Example — after:**
```
┌──────────────────────────────┐
│ Code Review Agent      v1.0  │
│ anthropic/code-review        │
│ [Published] [3 skills]       │
│                              │
│ An agent that reviews code   │
│ and suggests improvements... │
│                              │
│ #code-review  #security      │
│ #best-practices              │
└──────────────────────────────┘
```

**Scope:** Frontend-only. The skill tags are already in the API response under
`latest_version.skills[].tags`. Just render the first few unique tags.

> **Decision: Accepted**
>
>

---

### 2.6 Publisher name + verified badge on cards

**Problem:** Cards show `namespace/slug` (e.g., `anthropic/postgres-mcp`) but don't
show the publisher's display name or verified status. The namespace is a technical
identifier, not a trust signal. Users can't distinguish a verified company from a
random individual at a glance.

**What to do:** Add the publisher display name and a checkmark icon for verified
publishers on each card. This requires the list API to include publisher info in the
response (either embedded or via a join).

**Example — before:**
```
│ anthropic/postgres-mcp       │
```

**Example — after:**
```
│ postgres-mcp                 │
│ by Anthropic ✓               │
```

**Scope:** Requires the server list API to include publisher data (name, verified) in
the response, or a separate publisher lookup. Then a small UI change on the card
component.

> **Decision: Deferred**
>
> Good idea but not sure if it make sense at the moment

---

### 2.7 "Copy install command" button on cards

**Problem:** The most common user flow is: find a server → copy the install command →
paste into their MCP host config. Currently this requires clicking into the detail
page, finding the packages section, and manually copying. That's 3 steps for the most
frequent action.

**What to do:** Add a small "copy" icon button on each MCP server card. Clicking it
copies the primary install command (e.g., `npx @anthropic/postgres-mcp`) to the
clipboard and shows a brief "Copied!" toast.

**Example:**
```
┌─────────────────────────────────────┐
│ postgres-mcp              v1.2.0    │
│ by Anthropic ✓            [Published]│
│                                     │
│ PostgreSQL integration for MCP...   │
│                                     │
│ npm: @anthropic/postgres-mcp   [📋] │
│ Created Apr 5, 2026                 │
└─────────────────────────────────────┘
                                  ^ copy button
```

**Scope:** Frontend-only. Read the first package's `identifier` + `version` from the
existing API response and format it as a copy-able command.

> **Decision: Accepted**
>
> This really matters for server implementation. I want users to know how can they reach the server. I won't have that many use cases where they will install the MCP on their computer, but the option should be here

---

## 3. MCP Server Detail Page — Structured for Decision-Making

Current state: Title bar with badges, a flat metadata grid (runtime, protocol version,
dates, license), a packages section showing install/connection info, links to repo and
homepage, and a collapsible raw JSON viewer.

---

### 3.1 Tabbed layout: Overview / Installation / Versions / Raw JSON

**Problem:** All information is on a single scrolling page with no hierarchy. The
metadata grid, installation instructions, and raw JSON are all at the same level. Users
who come specifically to install (the majority) must scroll past metadata they don't
care about.

**What to do:** Organize the detail page into tabs:
- **Overview** — description, metadata grid, publisher info, links.
- **Installation** — packages, install commands, config snippets (see 3.4).
- **Versions** — version history timeline (see 3.5).
- **JSON** — raw API response viewer.

**Example:**
```
postgres-mcp                                          v1.2.0 [Published]
by Anthropic ✓
PostgreSQL integration for the Model Context Protocol

[ Overview ]  [ Installation ]  [ Versions ]  [ JSON ]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

(tab content here)
```

**Scope:** Frontend refactor of the detail page into a `Tabs` component (shadcn/ui
has one). No API changes. Content is reorganized, not created.

> **Decision: Accepted**
>
> Good idea, but I would like to have a more concrete example and/or more fields to add. We could add more informations in a v2 when we transition to a platform instead of only a registry

---

### 3.2 Capabilities section

**Problem:** The `capabilities` field exists in the MCP server version schema (JSONB
in the database, exposed in the API) but is not rendered anywhere in the UI. This
field describes what the server can do: which MCP features it supports (tools,
resources, prompts, sampling, etc.). This is critical information for a user deciding
whether to install.

**What to do:** Add a "Capabilities" block on the detail page (in the Overview tab if
using tabs). Parse the capabilities JSON and render it as labeled badges or a
structured list.

**Example:**
```
Capabilities
┌─────────────────────────────────────────────────────┐
│ [✓ Tools]  [✓ Resources]  [✓ Prompts]  [✗ Sampling] │
│                                                     │
│ Tools (5):                                          │
│   • query      — Execute SQL queries                │
│   • list_tables — List database tables              │
│   • describe   — Describe table schema              │
│   • insert     — Insert rows                        │
│   • migrate    — Run migrations                     │
│                                                     │
│ Resources (2):                                      │
│   • schema://  — Database schema introspection      │
│   • data://    — Table data access                  │
└─────────────────────────────────────────────────────┘
```

**Scope:** Frontend rendering of the existing `capabilities` JSONB. The exact shape
depends on what servers store — may need to handle free-form JSON gracefully (render
as key-value pairs if structure is unknown). No backend changes if the field is already
returned in the API.

> **Decision: Accepted**
>
> 

---

### 3.3 README / long description (markdown rendered)

**Problem:** Entries currently have a single `description` field which is a short
one-liner. Registries like npm, PyPI, crates.io, and Docker Hub all show a full README
with usage examples, screenshots, configuration docs, and changelog highlights. A
one-liner doesn't give users enough information to evaluate the entry.

**What to do:** Add a `readme` or `long_description` text field (Markdown) to the
entry schema. Render it on the detail page using a Markdown renderer (e.g.,
`react-markdown`). This becomes the main content of the Overview tab.

**Example:**
```
Overview tab:

# postgres-mcp

A Model Context Protocol server for PostgreSQL databases.

## Features
- Execute read/write SQL queries
- Introspect database schema
- Run migrations safely with rollback support

## Configuration
Set the `DATABASE_URL` environment variable:
  DATABASE_URL=postgres://user:pass@host:5432/dbname

## Usage
(examples with screenshots...)
```

**Scope:** Requires a new `readme` column in the database, API schema update, admin
form field for editing it, and a Markdown rendering component on the public detail
page. Medium effort.

> **Decision: Accepted**
>
>

---

### 3.4 One-click install snippet

**Problem:** Users come to the registry to install MCP servers. The current detail page
shows the package identifier and transport type, but doesn't generate a ready-to-paste
configuration block for any specific MCP host. Users must manually assemble the JSON
config themselves — error-prone and tedious.

**What to do:** Generate a copy-able config snippet for the most common MCP hosts.
For stdio-based servers, this is a JSON block with `command` and `args`. For
HTTP-based servers, it's the endpoint URL and auth config.

**Example — for a stdio npm package:**
```
Add to your MCP host configuration:

Claude Desktop / Cursor:
┌──────────────────────────────────────────────────┐
│ {                                           [📋] │
│   "mcpServers": {                                │
│     "postgres-mcp": {                            │
│       "command": "npx",                          │
│       "args": [                                  │
│         "-y",                                    │
│         "@anthropic/postgres-mcp@1.2.0"          │
│       ]                                          │
│     }                                            │
│   }                                              │
│ }                                                │
└──────────────────────────────────────────────────┘
```

**Example — for a remote HTTP server:**
```
Add to your MCP host configuration:

┌──────────────────────────────────────────────────┐
│ {                                           [📋] │
│   "mcpServers": {                                │
│     "stripe-mcp": {                              │
│       "url": "https://mcp.stripe.com/v1/sse",    │
│       "headers": {                               │
│         "Authorization": "Bearer <YOUR_TOKEN>"   │
│       }                                          │
│     }                                            │
│   }                                              │
│ }                                                │
└──────────────────────────────────────────────────┘
```

**Scope:** Frontend-only logic that reads the `packages[]` data from the API response
and generates config JSON. A `CodeBlock` component with a copy button. No backend
changes.

> **Decision: Accepted**
>
> stdio is important, but the MCP hosts are top priority

---

### 3.5 Version history

**Problem:** Only `latest_version` is shown on the detail page. Users have no way to
see how many versions have been published, when they were released, or whether the
project is actively maintained. A server with 12 versions over 6 months signals active
development; one with a single version from a year ago signals abandonment.

**What to do:** Add a "Versions" tab showing a chronological list of all published
versions with their version number, publish date, and status (published/deprecated).

**Example:**
```
Versions tab:

v1.2.0  — Published Apr 5, 2026     [Latest]
v1.1.0  — Published Mar 12, 2026
v1.0.0  — Published Jan 8, 2026
v0.9.0  — Deprecated Dec 1, 2025
v0.1.0  — Deprecated Oct 15, 2025
```

**Scope:** Requires a `GET /api/v1/mcp/servers/{ns}/{slug}/versions` endpoint (may
already exist or be planned). Frontend renders a version list component.

> **Decision: Accepted**
>
>

---

### 3.6 Related servers

**Problem:** After viewing a server's detail page, the user has no path to discover
similar entries. They must go back to the listing and search again. This breaks the
browsing flow and reduces discoverability.

**What to do:** Show a "Related Servers" section at the bottom of the detail page.
Relation can be based on:
- Same publisher / namespace (simplest).
- Same ecosystem (e.g., other npm-based MCP servers).
- Shared tags or categories (if tags exist, see 1.3).

**Example:**
```
Related Servers
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ mysql-mcp    │ │ sqlite-mcp  │ │ mongodb-mcp │
│ by Anthropic │ │ by Community│ │ by MongoDB   │
│ v1.0.0       │ │ v0.8.0      │ │ v2.1.0      │
└─────────────┘ └─────────────┘ └─────────────┘
```

**Scope:** Can start simple (same namespace query) with no backend changes. A smarter
version needs tags or a recommendation engine.

> **Decision: Accepted**
>
> 

---

### 3.7 Publisher card sidebar

**Problem:** The detail page shows `namespace/slug` but doesn't surface who the
publisher is, whether they're verified, or what else they've published. The publisher
is a key trust signal — users trust "Stripe's official MCP server" more than
"random-user's stripe server."

**What to do:** Add a sidebar card (or a section in the page header) showing:
- Publisher name and verified badge.
- Publisher contact (if public).
- Count of other published entries.
- Link to the publisher's profile page (see 7.2).

**Example:**
```
┌─ Publisher ───────────────┐
│  Anthropic  ✓ Verified    │
│                           │
│  12 MCP Servers           │
│   4 Agents                │
│                           │
│  [View all entries →]     │
└───────────────────────────┘
```

**Scope:** Requires the detail API to include publisher information (join or embed), or
a separate publisher fetch. Frontend adds a sidebar or header section.

> **Decision: Accepted**
>
>

---

## 4. Agent Detail Page — Make Skills the Star

Current state: Title bar with badges, metadata grid (endpoint URL, A2A protocol
version, dates, input/output modes, auth schemes), a skills list, links, and a raw
JSON viewer.

---

### 4.1 Skills as the primary content block

**Problem:** Skills are listed as a secondary section below the metadata grid. But
skills are the single most important piece of information about an agent — they define
what it can do. The metadata grid (protocol version, dates, auth) is supporting
context, not the main content.

**What to do:** Promote skills to be the primary visual block on the page. Each skill
gets a rich card with its name, description, tags as colored pills, and example
prompts. Place them above or at the same level as the metadata.

**Example:**
```
code-review-agent                                v1.0.0 [Published]
by Anthropic ✓
AI-powered code review agent

Skills (3)
┌──────────────────────────────────────────────────────────┐
│ Review Pull Request                                      │
│ Analyzes code changes and provides detailed feedback     │
│                                                          │
│ [code-review] [security] [best-practices]                │
│                                                          │
│ Examples:                                                │
│   "Review this PR for security issues"                   │
│   "Check if this change follows our coding standards"    │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ Generate Tests                                           │
│ Creates unit tests for the provided code                 │
│                                                          │
│ [testing] [unit-tests] [code-gen]                        │
│                                                          │
│ Examples:                                                │
│   "Write tests for the UserService class"                │
└──────────────────────────────────────────────────────────┘
```

**Scope:** Frontend restructuring of the agent detail page. No API changes — all data
is already in `latest_version.skills[]`.

> **Decision: Accepted**
>
>

---

### 4.2 "Try it" or example interaction section

**Problem:** The `examples` field on skills is currently rendered as a plain list of
strings. This doesn't convey the conversational nature of interacting with an agent.
Users want to see what a real interaction looks like before they invest time integrating.

**What to do:** Format examples as conversation-style prompt cards, visually distinct
from regular text. Optionally, if the agent has a public endpoint, add a "Try it"
button that opens a simple request form.

**Example:**
```
Example interactions:

┌─ You ────────────────────────────────────────────────┐
│ Review this PR for security issues and suggest fixes  │
└──────────────────────────────────────────────────────┘
            │
┌─ Agent ──────────────────────────────────────────────┐
│ I found 3 security issues in your PR:                │
│ 1. SQL injection risk in query.go:42                 │
│ 2. Missing input validation on /api/users endpoint   │
│ 3. Hardcoded secret in config.go:15                  │
│                                                      │
│ Suggested fixes: ...                                 │
└──────────────────────────────────────────────────────┘
```

**Scope:** Frontend-only for the display formatting. A "Try it" feature would require
proxying to the agent's endpoint, which is significantly more complex and may have
auth implications.

> **Decision: Deffered**
>
> Might be hard to implement with authentication and security challenges

---

### 4.3 Authentication guide

**Problem:** The detail page shows authentication schemes as raw badges
(`[Bearer] [OAuth2]`) but provides no guidance on how to actually authenticate. A
developer integrating this agent needs to know: what token to provide, where to get
it, and how to format the request.

**What to do:** Add a "How to connect" section that combines the endpoint URL with
auth setup instructions tailored to the declared scheme. For OAuth2, show the flow.
For Bearer, show the header format. For ApiKey, show where to place it.

**Example:**
```
How to connect

Endpoint: https://api.example.com/agent/v1       [📋]

Authentication: Bearer Token
┌──────────────────────────────────────────────────┐
│ curl -X POST https://api.example.com/agent/v1 \  │
│   -H "Authorization: Bearer <YOUR_TOKEN>" \      │
│   -H "Content-Type: application/json" \          │
│   -d '{"message": "Review my code"}'             │
└──────────────────────────────────────────────────┘

To obtain a token, follow the provider's authentication
documentation.
```

**Scope:** Frontend template logic based on the `authentication` array from the API.
Each scheme type maps to a different instruction template. No backend changes.

> **Decision: Accepted**
>
>

---

### 4.4 Input/Output modes explained

**Problem:** Input and output modes are displayed as raw JSON arrays:
`["text", "image"]`. This is technically accurate but not user-friendly. Non-technical
users or newcomers won't immediately understand what these modes mean in practice.

**What to do:** Replace raw arrays with icon + label pairs. Use recognizable icons for
each mode type and add a brief explanation of what each means.

**Example — before:**
```
Input modes:  ["text", "image", "file"]
Output modes: ["text"]
```

**Example — after:**
```
Accepts                          Returns
┌────────────────────────┐      ┌────────────────────────┐
│ 📝 Text                │      │ 📝 Text                │
│ 🖼️ Images              │      └────────────────────────┘
│ 📎 File attachments    │
└────────────────────────┘
```

**Scope:** Frontend-only. A small mapping component from mode strings to icons/labels.

> **Decision: Accepted**
>
> Good idea, might need more examples

---

### 4.5 Surface `icon_url`, `documentation_url`, `provider`, `status_message`

**Problem:** The `AgentVersion` schema includes several fields that are stored in the
database and returned by the API but never rendered in the UI:
- `icon_url` — the agent's visual identity (logo/avatar).
- `documentation_url` — link to external docs.
- `provider` — the organization/individual that runs the agent.
- `status_message` — operational status (e.g., "beta", "maintenance window").

These are all high-value fields that users want to see.

**What to do:**
- `icon_url`: Render as the agent's avatar in the page header and on cards.
- `documentation_url`: Show as a prominent "Documentation" link next to the title.
- `provider`: Display in a "Provider" metadata field (distinct from publisher).
- `status_message`: Show as an alert banner at the top of the detail page if present.

**Example:**
```
┌─ ⚠️ Status ──────────────────────────────────────────┐
│ This agent is in beta. Expect breaking changes.       │
└──────────────────────────────────────────────────────┘

[Agent Icon]  Code Review Agent           v1.0.0
              by Anthropic ✓
              Provider: Anthropic Cloud Services
              📖 Documentation  |  🔗 Endpoint
```

**Scope:** Frontend changes to render existing API fields. No backend changes needed
if these fields are already in the version response. Verify they are included in the
`latest_version` projection — if not, the API endpoint may need updating.

> **Decision: Accepted**
>
> _your comment here_

---

### 4.6 A2A Agent Card preview

**Problem:** The detail page has a link to download the A2A Agent Card JSON, but
developers integrating via the A2A protocol want to preview the card structure without
downloading a file. They want to verify the card is well-formed and contains the
expected fields before pointing their client at it.

**What to do:** Replace the plain download link with an inline preview panel showing
the formatted Agent Card JSON with syntax highlighting and a copy button. Optionally,
show a "card anatomy" view that labels each field.

**Example:**
```
A2A Agent Card                               [📋 Copy] [⬇ Download]
┌──────────────────────────────────────────────────────────────────┐
│ {                                                                │
│   "name": "Code Review Agent",                                   │
│   "description": "AI-powered code review agent",                 │
│   "url": "https://api.example.com/agent/v1",                     │
│   "version": "1.0.0",                                            │
│   "capabilities": {                                              │
│     "streaming": true,                                           │
│     "pushNotifications": false                                   │
│   },                                                             │
│   "skills": [                                                    │
│     {                                                            │
│       "id": "review-pr",                                         │
│       "name": "Review Pull Request",                             │
│       "description": "Analyzes code changes..."                  │
│     }                                                            │
│   ],                                                             │
│   "defaultInputModes": ["text"],                                 │
│   "defaultOutputModes": ["text"]                                 │
│ }                                                                │
└──────────────────────────────────────────────────────────────────┘
```

**Scope:** Frontend component. Fetch the agent card endpoint
(`/agents/{ns}/{slug}/.well-known/agent-card.json`) and render it in a syntax-
highlighted code block. The endpoint already exists.

> **Decision: Rejected (low priority)**
>
> The JSON link is sufficient for now. An inline preview is a convenience
> but not worth the effort given the existing download link works fine.

---

## 5. Cross-Cutting UX Improvements

---

### 5.1 Breadcrumb navigation

**Problem:** Detail pages have no breadcrumbs. When a user navigates from the listing
to a detail page, the browser's back button is their only way to return. They also
lose visual context of where they are in the site hierarchy.

**What to do:** Add a breadcrumb bar below the header on all non-home pages.

**Example:**
```
Home  >  MCP Servers  >  anthropic  >  postgres-mcp
```
```
Home  >  Agents  >  anthropic  >  code-review-agent
```
```
Home  >  Admin  >  Publishers  >  anthropic
```

**Scope:** A reusable `Breadcrumb` component (shadcn/ui has one). Each page passes its
breadcrumb segments. No API changes.

> **Decision: Accepted**
>
>

---

### 5.2 Loading skeletons

**Problem:** When data is loading (TanStack Query fetching), the page likely shows
either nothing or a spinner. This causes a flash of empty content followed by a
sudden pop-in of cards, which feels jarring and slow.

**What to do:** Show skeleton placeholders that match the shape of the real content
while loading. For card grids, show grey pulsing rectangles in the card shape. For
detail pages, show skeleton bars for each metadata field.

**Example — listing skeleton:**
```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ ████████    │ │ ████████    │ │ ████████    │
│ ██████      │ │ ██████      │ │ ██████      │
│             │ │             │ │             │
│ ████████████│ │ ████████████│ │ ████████████│
│ ████████    │ │ ████████    │ │ ████████    │
└─────────────┘ └─────────────┘ └─────────────┘
   (pulsing animation)
```

**Scope:** Skeleton components using shadcn/ui's `Skeleton` primitive. One for cards,
one for detail pages, one for tables. Frontend-only.

> **Decision: Accepted**
>
> Yes good idea

---

### 5.3 Empty state illustrations

**Problem:** When filters produce zero results, or a section has no data, the user
sees a blank area with no guidance. This is a dead-end — the user doesn't know if the
system is broken, if their filters are too narrow, or what to do next.

**What to do:** Show contextual empty states with an illustration, a message, and an
action. Tailor the message to the context.

**Example — no search results:**
```
      ┌───────────┐
      │   🔍 ?    │
      └───────────┘
  No servers match "foobar"

  Try a different search term or
  [Clear all filters]
```

**Example — empty agent skills:**
```
      ┌───────────┐
      │   ⚡ 0    │
      └───────────┘
  No skills defined yet

  This agent hasn't published any
  skills in its latest version.
```

**Scope:** A reusable `EmptyState` component with props for icon, title, description,
and action. Frontend-only.

> **Decision: Accepted**
>
>

---

### 5.4 Deep-linkable filters

**Problem:** Filter state (search query, namespace, status, etc.) is stored in
component state and lost when the user copies the URL or refreshes the page. Users
can't share a filtered view ("here are all the published npm MCP servers") with a
colleague.

**What to do:** Sync all filter values to URL search parameters. When the page loads,
read initial filter values from the URL. When filters change, update the URL without
a full page reload.

**Example:**
```
/mcp?q=postgres&status=published&transport=stdio

Sharing this URL with someone else opens the MCP listing page
pre-filtered to published stdio servers matching "postgres".
```

**Scope:** Use React Router's `useSearchParams` hook (already available). Replace
local state with URL-derived state for each filter. Frontend-only.

> **Decision: Accepted**
>
>

---

### 5.5 Keyboard shortcuts

**Problem:** Developers are keyboard-driven. The current UI is entirely mouse-driven —
there's no way to quickly focus the search bar, navigate between cards, or access
common actions without clicking.

**What to do:** Add keyboard shortcuts for common actions:
- `/` — focus the search input (global, like GitHub).
- `j` / `k` — move focus down/up through card or table rows.
- `Enter` — open the focused card's detail page.
- `Escape` — clear search / close modals.
- `?` — show a keyboard shortcuts help modal.

**Example:**
```
Press ? for keyboard shortcuts

┌─ Keyboard Shortcuts ─────────────────────┐
│                                          │
│  /         Focus search                  │
│  j / k     Navigate entries              │
│  Enter     Open selected entry           │
│  Escape    Clear / close                 │
│  g h       Go to home                    │
│  g m       Go to MCP servers             │
│  g a       Go to agents                  │
│                                          │
└──────────────────────────────────────────┘
```

**Scope:** A custom `useKeyboardShortcuts` hook + a help modal. Moderate effort.
Requires managing focus state on card grids.

> **Decision: Rejected**
>
> Over-engineered for a registry. Only the `/` to focus search shortcut
> might be worth adding as a one-liner — no full shortcut system needed.

---

### 5.6 Dark mode polish

**Problem:** The app has a `ThemeProvider` for light/dark mode switching, but badges,
code blocks, and card borders may not be tuned for both themes. Common issues: low
contrast badges in dark mode, invisible borders, code blocks with wrong background.

**What to do:** Audit every component in both themes. Specifically check:
- Status badges (draft/published/deprecated) contrast.
- Code blocks and JSON viewers background + text colors.
- Card borders and shadows.
- Input fields and dropdowns.
- Skeleton animation colors.

**Example of a common issue:**
```
Light mode:  [Published] ← green badge, white text, looks great
Dark mode:   [Published] ← same green badge, now barely visible
                           against dark background
Fix:         Use Tailwind's dark: variant to adjust badge colors
```

**Scope:** CSS/Tailwind audit and fixes. No logic changes.

> **Decision: Accepted**
>
> 

---

### 5.7 Responsive detail pages

**Problem:** Detail pages use grid layouts for metadata and packages that may not
reflow properly on narrow screens. Developers do browse registries on mobile phones
(during debugging, on the go, sharing links in chat).

**What to do:** Test all detail pages at 375px width (iPhone SE) and 768px (tablet).
Fix any overflow, ensure metadata grids stack to single column, and verify code blocks
have horizontal scrolling.

**Example of a common issue:**
```
Desktop (works):
Runtime: stdio   |   Protocol: 2024-11-05   |   License: MIT

Mobile (broken — overflows):
Runtime: stdio   |   Protocol: 2024-11-05   |   Licen

Mobile (fixed — stacks):
Runtime:    stdio
Protocol:   2024-11-05
License:    MIT
```

**Scope:** Tailwind responsive class adjustments (`sm:`, `md:`, `lg:` breakpoints).
No logic changes.

> **Decision: Accepted**
>
> Mobile UX is very bad at the moment, this is a good idea

---

## 6. Admin-Specific Improvements

---

### 6.1 Dashboard with real metrics

**Problem:** The admin dashboard currently has limited actionable information. Admins
need at-a-glance understanding of registry health: what needs attention, what's stale,
and what's been happening.

**What to do:** Build a dashboard with:
- **Status breakdown** — pie or bar chart: N draft, N published, N deprecated.
- **Recent activity** — timeline of recent creates/updates/publishes.
- **Action items** — entries with no description, drafts older than 30 days, entries
  from unverified publishers.
- **Quick stats** — total entries, total publishers, verified ratio.

**Example:**
```
Admin Dashboard

┌─ At a Glance ────────────────────────────────────────┐
│ 47 MCP Servers  │  23 Agents  │  12 Publishers       │
│ 38 published    │  18 published│  8 verified          │
│  6 drafts       │   4 drafts  │                      │
│  3 deprecated   │   1 deprecated│                    │
└──────────────────────────────────────────────────────┘

⚠ Needs attention:
  • 3 drafts older than 30 days (stale?)
  • 5 entries with no description
  • 2 publishers pending verification

Recent activity:
  • Apr 12 — anthropic/postgres-mcp v1.2.0 published
  • Apr 11 — new publisher "acme-corp" created
  • Apr 10 — anthropic/slack-mcp deprecated
```

**Scope:** Requires aggregate API endpoints (counts by status, recent activity log).
Frontend dashboard components with charts (e.g., recharts or a simple bar).

> **Decision: Accepted**
>
> For a v2 (when we transit from registry to platform) we will also add metrics from the Tool Gateway (which will route every MCP and Agent calls). This is a very first implementation

---

### 6.2 Inline status workflow

**Problem:** Status transitions (Draft → Published → Deprecated) are handled by
separate buttons scattered across the page. The admin has to mentally track which
transitions are valid and find the right button. There's no visual representation of
the lifecycle.

**What to do:** Replace individual buttons with a visual state machine / stepper
component showing the lifecycle. The current state is highlighted, and valid
transitions are shown as clickable next steps.

**Example:**
```
Lifecycle:

  ● Draft  ──────>  ○ Published  ──────>  ○ Deprecated
  (current)         [Publish →]

After publishing:

  ✓ Draft  ──────>  ● Published  ──────>  ○ Deprecated
                    (current)             [Deprecate →]
```

**Scope:** A reusable `LifecycleStepper` component. Frontend-only — the API
transitions already exist as individual endpoints.

> **Decision: Accepted**
>
>

---

### 6.3 Bulk actions on list pages

**Problem:** If an admin needs to deprecate 10 old servers or change the visibility of
5 entries, they must do it one by one — open each detail page, click the action,
confirm, go back, repeat. This is painful for registry maintenance tasks.

**What to do:** Add row checkboxes on admin list pages and a bulk action bar that
appears when items are selected. Supported actions: change status, change visibility,
delete.

**Example:**
```
┌ 3 selected ──────────────────────────────────────────┐
│  [Set Published]  [Set Deprecated]  [Make Private]   │
│  [Delete]                              [Clear selection] │
└──────────────────────────────────────────────────────┘

☑ postgres-mcp        anthropic   v1.2.0   Published
☑ mysql-mcp           anthropic   v1.0.0   Published
☐ github-mcp          anthropic   v2.0.1   Published
☑ old-server          legacy      v0.5.0   Draft
☐ redis-mcp           community   v0.3.0   Draft
```

**Scope:** Frontend state management for selection + a bulk action API (or sequential
calls to existing individual endpoints). If using sequential calls, add loading
indicators and error handling per item.

> **Decision: Acepted**
>
>

---

### 6.4 Publisher association on create forms

**Problem:** When an admin creates a new MCP server or agent, the publisher
relationship is set implicitly through the namespace field. The admin has to know
which namespace maps to which publisher. There's no visual confirmation of "you're
creating this under Publisher X."

**What to do:** Replace the free-text namespace input with a publisher dropdown. When
a publisher is selected, auto-fill the namespace and show the publisher's name and
verified status. This makes the relationship explicit and prevents typos.

**Example — before:**
```
Create MCP Server

Name:       [_______________]
Namespace:  [_______________]    ← what do I type here?
Slug:       [_______________]
```

**Example — after:**
```
Create MCP Server

Publisher:  [ Select publisher  ▾ ]
            ┌───────────────────────┐
            │ Anthropic  ✓          │
            │ Acme Corp             │
            │ Community  ✓          │
            └───────────────────────┘

Name:       [_______________]
Slug:       [_______________]    ← auto-suggested from name
Namespace:  anthropic            ← auto-filled from publisher
```

**Scope:** Frontend change to create forms. Requires a publishers list API call to
populate the dropdown (the admin publishers endpoint already exists).

> **Decision: Deferred**
>
> This is a good idea, and I accept this as a first implementation. However I think that we will transition to a project/team based instead of Publisher (or maybe both) in a v2

---

## 7. Information Architecture & Navigation

---

### 7.1 Unified "Explore" page

**Problem:** The current navigation separates MCP servers and agents into distinct
pages. But many users — especially newcomers — don't know whether the tool they need
is packaged as an MCP server or an agent. Forcing a choice upfront creates friction.

**What to do:** Create an `/explore` page that searches and displays both resource
types in a single view with a type filter (All / MCP Servers / Agents). The home page
search (proposal 1.2) would link here for full results.

**Example:**
```
Explore

[ All ]  [ MCP Servers ]  [ Agents ]

Search: [postgres________________]

Results (5):

  🔌 postgres-mcp          MCP Server   v1.2.0  Published
     PostgreSQL integration for MCP

  🤖 db-migration-agent    Agent        v1.0.0  Published
     Automated database migration agent

  🔌 supabase-postgres     MCP Server   v0.9.1  Published
     Supabase PostgreSQL connector

  ...
```

**Scope:** New page component, combined query to both listing endpoints (parallel
fetch), unified result rendering. May benefit from a backend unified search endpoint
in the future.

> **Decision: Accepted**
>
>

---

### 7.2 Publisher public profile pages

**Problem:** Publishers exist only in the admin section. On the public side, users see
namespaces on cards but can't click through to see who the publisher is or what else
they've built. This is a missed opportunity for discoverability and trust-building.

**What to do:** Create a public `/publishers/{slug}` page showing:
- Publisher name, verified status, contact (if public), description/bio.
- All published MCP servers by this publisher.
- All published agents by this publisher.

**Example:**
```
/publishers/anthropic

Anthropic                                    ✓ Verified
Building reliable, interpretable AI systems

MCP Servers (12)
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ postgres-mcp │ │ github-mcp  │ │ slack-mcp   │
└─────────────┘ └─────────────┘ └─────────────┘
[View all 12 →]

Agents (4)
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ code-review  │ │ test-gen    │ │ doc-writer  │
└─────────────┘ └─────────────┘ └─────────────┘
[View all 4 →]
```

**Scope:** New public API endpoint (`GET /api/v1/publishers/{slug}` — may need to be
added) + new page component. Entries can be fetched using existing list endpoints with
a `namespace` filter.

> **Decision: Accepted**
>
> Keep in mind that we will add project/teams in a v2 in addition to publishers

---

### 7.3 Namespace as a first-class navigation concept

**Problem:** Namespaces appear on every card (`anthropic/postgres-mcp`) but aren't
clickable. They're a display-only text element. Users intuitively want to click them
to see "all entries in this namespace" — a pattern familiar from GitHub, npm, and
Docker Hub.

**What to do:** Make the namespace on cards and detail pages a clickable link. It
should navigate to the listing page pre-filtered by that namespace (using deep-linkable
filters from proposal 5.4).

**Example:**
```
Card: anthropic/postgres-mcp
                ^^^^^^^^^
                clickable → navigates to /mcp?namespace=anthropic

Detail page:
  Home > MCP Servers > anthropic > postgres-mcp
                       ^^^^^^^^^
                       clickable → navigates to /mcp?namespace=anthropic
```

**Scope:** Minimal — wrap existing namespace text in a `<Link>` component with the
appropriate query parameter. Frontend-only.

> **Decision: Accepted**
>
> Keep in mind that we will add project/teams in a v2 in addition to publishers

---

### 7.4 Sitemap / registry overview page

**Problem:** There's no way to get a bird's-eye view of the entire registry. Users who
want to understand the breadth of available entries must paginate through listing pages.
Search engines also benefit from a structured overview for indexing.

**What to do:** Create an `/overview` or `/sitemap` page that lists all namespaces
with their entry counts, grouped alphabetically or by category. Each namespace links
to its filtered listing.

**Example:**
```
Registry Overview

A
  acme-corp (3 servers, 1 agent)
  aws-community (7 servers)

C
  cloudflare (2 servers, 2 agents)
  community (15 servers, 8 agents)

D
  datadog (3 servers, 3 agents)
  ...

Total: 47 MCP Servers, 23 Agents across 12 publishers
```

**Scope:** A new page fetching namespace/publisher aggregates. May need a dedicated
API endpoint for namespace listing with counts.

> **Decision: Deferred (very low priority)**
>
> Not needed at current registry size. Revisit only if SEO becomes a
> concern or the registry grows to hundreds of namespaces.

---

## 8. Developer Experience (DX) Features

---

### 8.1 API playground / interactive docs

**Problem:** Developers integrating with the registry API have to read the OpenAPI
spec and manually construct curl commands or write client code. There's no interactive
way to explore the API from the browser.

**What to do:** Serve Swagger UI, Scalar, or Redoc at `/docs` or `/api-docs`, backed
by the existing `openapi.yaml` spec. Scalar has the most modern UI and supports
try-it-out requests.

**Example:**
```
/docs

┌─ AI Registry API ────────────────────────────────────┐
│                                                      │
│ GET /api/v1/mcp/servers                              │
│ List all MCP servers                                 │
│                                                      │
│ Parameters:                                          │
│   q:         [postgres     ]                         │
│   status:    [published  ▾ ]                         │
│   limit:     [10           ]                         │
│                                                      │
│ [Try it out]                                         │
│                                                      │
│ Response:                                            │
│ ┌──────────────────────────────────────────────────┐│
│ │ { "items": [...], "total_count": 3 }             ││
│ └──────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────┘
```

**Scope:** Near-zero effort if using a hosted library. Add Scalar or Swagger UI as a
static page pointing at `/openapi.yaml` (already served). Can be a single HTML file.

> **Decision: Already implemented — closed**
>
> Scalar is already fully set up: served at `/docs` via CDN, backed by
> the embedded `/openapi.yaml` spec, with "purple" theme. Proxied through
> nginx. No action needed.

---

### 8.2 Copy-friendly identifiers everywhere

**Problem:** Developers constantly copy namespace/slug pairs, version strings, endpoint
URLs, and package identifiers from the UI into their configs, code, and terminals.
Currently they must manually select text and copy — often getting trailing whitespace,
badge text, or adjacent content in the selection.

**What to do:** Add a small clipboard icon button next to every copiable value. On
click, copy the value and show a brief "Copied!" feedback. Target values:
- `namespace/slug` on cards and detail pages.
- Version strings.
- Endpoint URLs.
- Package identifiers.
- Any code snippet or JSON block.

**Example:**
```
Endpoint:  https://api.example.com/agent/v1   [📋]
Version:   1.2.0                              [📋]
Package:   @anthropic/postgres-mcp            [📋]
```

**Scope:** A reusable `CopyButton` component. Wrap each copiable field. Frontend-only,
small effort, high frequency of use.

> **Decision: Accepted**
>
>

---

### 8.3 MCP config generator

**Problem:** Different MCP hosts (Claude Desktop, Cursor, Windsurf, VS Code + Cline,
etc.) have slightly different configuration formats. The current detail page shows raw
package metadata but doesn't generate a host-specific config block. Users must read the
host's docs and manually assemble the config.

**What to do:** Add a widget on the MCP server detail page with a host selector
dropdown. Based on the selected host and the server's package data, generate the
exact config block the user needs to paste.

**Example:**
```
Configure for:  [ Claude Desktop ▾ ]

┌──────────────────────────────────────────────────┐
│ // Add to claude_desktop_config.json             │
│ {                                           [📋] │
│   "mcpServers": {                                │
│     "postgres-mcp": {                            │
│       "command": "npx",                          │
│       "args": [                                  │
│         "-y",                                    │
│         "@anthropic/postgres-mcp@1.2.0"          │
│       ],                                         │
│       "env": {                                   │
│         "DATABASE_URL": "<your-database-url>"    │
│       }                                          │
│     }                                            │
│   }                                              │
│ }                                                │
└──────────────────────────────────────────────────┘

Configure for:  [ Cursor ▾ ]

┌──────────────────────────────────────────────────┐
│ // Add to .cursor/mcp.json                  [📋] │
│ {                                                │
│   "mcpServers": {                                │
│     "postgres-mcp": {                            │
│       "command": "npx",                          │
│       "args": ["-y", "@anthropic/postgres-mcp"]  │
│     }                                            │
│   }                                              │
│ }                                                │
└──────────────────────────────────────────────────┘
```

**Scope:** Frontend component with a host config template map. Reads package data
from the API response and fills in the template. May need a small data file mapping
host names to their config format and file paths.

> **Decision: Accepted**
>

---

### 8.4 Agent connection snippet generator

**Problem:** Developers integrating an agent need to know exactly how to call its A2A
endpoint. The current detail page shows the endpoint URL and auth scheme, but the
developer still has to write the HTTP request themselves.

**What to do:** Generate ready-to-use code snippets in multiple languages showing how
to call the agent. A tab selector lets the user switch between languages.

**Example:**
```
Connect to this agent:

[ curl ]  [ Python ]  [ TypeScript ]  [ Go ]

curl:
┌──────────────────────────────────────────────────┐
│ curl -X POST \                              [📋] │
│   https://api.example.com/agent/v1 \             │
│   -H "Authorization: Bearer <TOKEN>" \           │
│   -H "Content-Type: application/json" \          │
│   -d '{                                          │
│     "jsonrpc": "2.0",                            │
│     "method": "tasks/send",                      │
│     "params": {                                  │
│       "message": {                               │
│         "role": "user",                          │
│         "parts": [{"text": "Hello, agent"}]      │
│       }                                          │
│     }                                            │
│   }'                                             │
└──────────────────────────────────────────────────┘

Python:
┌──────────────────────────────────────────────────┐
│ import httpx                                [📋] │
│                                                  │
│ resp = httpx.post(                               │
│     "https://api.example.com/agent/v1",          │
│     headers={"Authorization": "Bearer <TOKEN>"}, │
│     json={                                       │
│         "jsonrpc": "2.0",                        │
│         "method": "tasks/send",                  │
│         "params": {                              │
│             "message": {                         │
│                 "role": "user",                   │
│                 "parts": [{"text": "Hello"}]     │
│             }                                    │
│         }                                        │
│     }                                            │
│ )                                                │
└──────────────────────────────────────────────────┘
```

**Scope:** Frontend template component with language tabs. Templates are parameterized
with the agent's endpoint URL and auth scheme. No backend changes.

> **Decision: Accepted**
>
>

---

### 8.5 Diff between versions

**Problem:** When a new version of an MCP server or agent is published, users want to
know what changed. Did capabilities get added? Were skills removed? Did the endpoint
URL change? Currently there's no way to compare versions.

**What to do:** On the version history view (proposal 3.5), add a "Compare" feature.
Select two versions and see a side-by-side or unified diff of their metadata, packages,
capabilities, and skills.

**Example:**
```
Compare:  [ v1.1.0 ▾ ]  ↔  [ v1.2.0 ▾ ]

┌─ Changes ────────────────────────────────────────────┐
│                                                      │
│ + Added capability: prompts                          │
│ ~ Updated package: @anthropic/postgres-mcp           │
│     version: 1.1.0 → 1.2.0                          │
│ + Added tool: migrate                                │
│   "Run database migrations safely"                   │
│ - Removed tool: unsafe_query                         │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**Scope:** Requires a version detail API that returns full version data (not just
latest). Frontend diff logic to compare two JSON objects and render the changes.
Moderate effort.

> **Decision: Accepted**
>
> Decision is accepted only if this can be automatically generated when bumping the versions

---

### 8.6 "Report an issue" link on detail pages

**Problem:** If a user finds a broken MCP server (endpoint down, outdated metadata,
security concern), there's no feedback mechanism. They have no way to alert the
registry maintainers without leaving the site.

**What to do:** Add a "Report an issue" link on every detail page. This can link to:
- A GitHub issue template (simplest — no backend needed).
- An internal form that creates a flagged entry in the admin dashboard.

**Example:**
```
┌──────────────────────────────────────────────────┐
│ postgres-mcp                v1.2.0  [Published]  │
│ ...                                              │
│                                                  │
│  ⚑ Report an issue with this entry               │
└──────────────────────────────────────────────────┘

Clicking opens:
┌─ Report Issue ──────────────────────────────────────┐
│                                                     │
│ Issue type:  [ Broken endpoint ▾ ]                  │
│              ┌─────────────────────┐                │
│              │ Broken endpoint     │                │
│              │ Outdated metadata   │                │
│              │ Security concern    │                │
│              │ Spam / abuse        │                │
│              │ Other               │                │
│              └─────────────────────┘                │
│                                                     │
│ Description: [_________________________________]    │
│                                                     │
│ [Submit]                                            │
└─────────────────────────────────────────────────────┘
```

**Scope:** Simplest version: a link to a pre-filled GitHub issue template
(`/issues/new?template=report&title=...`). No backend. Richer version: a report
endpoint + admin review queue.

> **Decision: Accepted**
>
> Accepted only if there is an admin review queue

---

## 9. Trust & Quality Signals

---

### 9.1 Verified badge on entries (not just publishers)

**Problem:** Publishers can be verified, but individual entries cannot. A verified
publisher might have 20 servers — some mature and tested, others experimental. Users
have no signal for entry-level quality beyond the publisher's reputation.

**What to do:** Add an optional `verified` or `certified` flag on entries that admins
can toggle. This indicates the registry maintainers have validated the entry works
correctly. Display it as a badge on cards and detail pages.

**Example:**
```
┌─────────────────────────────────────┐
│ postgres-mcp              v1.2.0    │
│ by Anthropic ✓                      │
│ [Published] [✓ Verified]            │
│                                     │
│ PostgreSQL integration for MCP...   │
└─────────────────────────────────────┘
```

**Scope:** New `verified` boolean column on the MCP server and agent tables. API
schema update. Admin toggle. Frontend badge rendering.

> **Decision: Accepted**
>
> 

---

### 9.2 Last-updated / freshness indicator

**Problem:** A server published 2 years ago with no updates may be abandoned. A server
updated last week is actively maintained. The current UI shows `created_at` and
`updated_at` as formatted dates, but doesn't interpret them. Users must do mental math
to gauge freshness.

**What to do:** Show a relative time label ("Updated 3 days ago") and a visual
freshness indicator. Optionally, flag stale entries (not updated in 6+ months) with a
subtle warning.

**Example:**
```
Active (updated < 3 months):
  Updated 3 days ago                              (green dot)

Aging (updated 3-12 months):
  Updated 8 months ago                            (yellow dot)

Stale (updated > 12 months):
  ⚠ Last updated 14 months ago — may be unmaintained  (red dot)
```

**Scope:** Frontend date formatting utility + a color-coded indicator component. No
API changes — uses existing `updated_at` field.

> **Decision: Accepted**
>
>

---

### 9.3 Compatibility matrix

**Problem:** MCP and A2A are evolving protocols with multiple versions. The detail page
shows `protocol_version` as a single string, but doesn't convey whether the server
works with the user's MCP host version. A server built for MCP protocol `2024-11-05`
may or may not work with a host running `2025-03-15`.

**What to do:** Display protocol compatibility as a visual matrix or as labeled badges
showing which protocol versions are supported. Link to the relevant spec version.

**Example:**
```
Compatibility:

  MCP Protocol:  2024-11-05  ✓ Current
  Transport:     stdio, HTTP
  Tested with:   Claude Desktop 1.x, Cursor 0.40+
```

**Scope:** Requires a `compatibility` or `tested_with` metadata field (new). Frontend
rendering is straightforward once the data exists. Could start with just the protocol
version display and expand later.

> **Decision: Accepted**
>
>

---

### 9.4 Health / uptime indicator for remote endpoints

**Problem:** Remote MCP servers and agents expose HTTP endpoints. These can go down
without the registry knowing. A user who installs a remote server only to find it
returns 503 has a bad experience and loses trust in the registry.

**What to do:** Run a periodic backend health check job that pings remote endpoints
and records their status. Display a green/yellow/red dot on the card and detail page.

**Example:**
```
┌─────────────────────────────────────┐
│ stripe-mcp              v2.0.1     │
│ [Published]  [HTTP]  🟢 Healthy    │
│                                     │
│ Stripe API integration for MCP...  │
│                                     │
│ Endpoint: https://mcp.stripe.com   │
│ Uptime: 99.9% (last 30 days)      │
└─────────────────────────────────────┘
```

**Scope:** Significant backend work: a health check worker, a status history table,
and a new API field. Frontend rendering is simple. Consider starting with a simple
"last checked" timestamp and HTTP status code before building full uptime tracking.

> **Decision: Accepted**
>
> In a v2 we might directly get this information from the Tool Gateway

---

### 9.5 Community signals

**Problem:** When multiple entries solve the same problem (e.g., 3 different Postgres
MCP servers), users have no data-driven way to choose between them. There are no
usage metrics, ratings, or community feedback signals.

**What to do:** Track and display aggregate usage signals. Options (from simplest to
most complex):
- **View count** — how many times the detail page was viewed.
- **Copy count** — how many times the install command was copied.
- **Star / bookmark** — let authenticated users star entries (requires user accounts).
- **Rating** — let users rate entries 1-5 (requires user accounts + review system).

**Example (simplest — view/copy counts):**
```
┌─────────────────────────────────────┐
│ postgres-mcp              v1.2.0    │
│ 👁 1,234 views   📋 567 installs    │
│                                     │
│ PostgreSQL integration for MCP...   │
└─────────────────────────────────────┘
```

**Scope:** View counts: simple backend counter increment on detail page load. Copy
counts: frontend event sent to a tracking endpoint on copy. Stars/ratings: requires
user identity, which the public UI currently doesn't have. Start with anonymous
counters.

> **Decision: Accepted (start simple)**
>
> Start with anonymous counters only — no user accounts needed:
> - `view_count` integer column on each entry, incremented via a
>   fire-and-forget POST on detail page load.
> - `copy_count` integer column, incremented when the install command is
>   copied.
> Total: 2 new columns, 2 lightweight endpoints, 2 frontend event calls.
> Stars/ratings deferred until user accounts exist.

---

## 10. Content & Onboarding

---

### 10.1 "What is MCP?" / "What is A2A?" explainer sections

**Problem:** Not every visitor knows what the Model Context Protocol or Agent-to-Agent
protocol is. A developer landing on the registry from a search engine may bounce
immediately if they don't understand what they're looking at.

**What to do:** Add brief, well-designed explainer sections. Options:
- Collapsible "What is MCP?" panel on the home page.
- A dedicated `/learn` or `/about` page with protocol overviews.
- Tooltips on the navigation items ("MCP Servers" → hover shows a 1-line explanation).

**Example — on the home page:**
```
┌─ What is MCP? ───────────────────────────────────────┐
│                                                      │
│ The Model Context Protocol (MCP) lets AI assistants  │
│ connect to external tools and data sources. An MCP   │
│ server exposes capabilities (tools, resources,       │
│ prompts) that any MCP-compatible host can use.       │
│                                                      │
│  ┌──────┐    MCP     ┌──────────┐                    │
│  │ Host │ ◄────────► │ Server   │                    │
│  │(Claude│           │(postgres)│                    │
│  └──────┘            └──────────┘                    │
│                                                      │
│ [Learn more →]                                       │
└──────────────────────────────────────────────────────┘
```

**Scope:** Static content page or component. No backend changes. Could link to the
official MCP and A2A spec sites for deep dives.

> **Decision: Accepted**
>
>

---

### 10.2 "Getting Started" guide

**Problem:** A new user lands on the registry, finds a server they want, but doesn't
know how to actually use it. The registry shows metadata but doesn't guide the user
through the full workflow from discovery to running the server.

**What to do:** Add a step-by-step "Getting Started" guide, either as a dedicated page
or as inline guidance on the detail page.

**Example:**
```
Getting Started with MCP Servers

Step 1: Find a server
  Use the search bar or browse categories to find
  a server that fits your needs.

Step 2: Copy the configuration
  Click the copy button on the server's detail page
  to get the config for your MCP host.

Step 3: Add to your host
  Paste the configuration into your host's config file:
  • Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json
  • Cursor: .cursor/mcp.json
  • VS Code + Cline: .vscode/mcp.json

Step 4: Start using it
  Restart your host and the server's tools will be
  available in your conversations.
```

**Scope:** Static content page. No backend changes. Could be contextually linked from
the home page hero and from detail pages.

> **Decision: Accepted (static page)**
>
> Implement as a hand-written static Markdown page — no backend needed.
> Link it from the home page hero and from detail pages contextually.

---

### 10.3 Contextual tooltips on technical fields

**Problem:** Technical fields like "runtime: stdio", "transport: SSE", and
"protocol_version: 2024-11-05" are displayed without explanation. Users unfamiliar
with MCP/A2A internals don't know what these mean or why they matter.

**What to do:** Add hover tooltips (or small info icons with popover) on technical
fields throughout the UI.

**Example:**
```
Runtime:   stdio  ⓘ
                  ┌──────────────────────────────────────┐
                  │ stdio: The server runs as a local    │
                  │ process on your machine. Your MCP    │
                  │ host starts and communicates with    │
                  │ it via stdin/stdout.                 │
                  └──────────────────────────────────────┘

Transport: SSE  ⓘ
                  ┌──────────────────────────────────────┐
                  │ SSE (Server-Sent Events): The server │
                  │ is hosted remotely. Your MCP host    │
                  │ connects via HTTP and receives       │
                  │ streaming responses.                 │
                  └──────────────────────────────────────┘
```

**Scope:** A tooltip data map (field name → explanation string) and a `TooltipInfo`
wrapper component. Use shadcn/ui's `Tooltip` primitive. Frontend-only.

> **Decision: Accepted**
>
>

---

### 10.4 Changelog / "What's new" feed

**Problem:** Returning users have no way to see what's new in the registry since their
last visit. They must manually browse listings and compare against their memory.

**What to do:** Add a "What's New" page or section showing a reverse-chronological
feed of registry events: new entries published, new publishers verified, entries
deprecated.

**Example:**
```
/changelog

What's New

Apr 12, 2026
  🆕 anthropic/postgres-mcp v1.2.0 published
  🆕 stripe/payments-agent v1.0.0 published
  ✓  Publisher "Stripe" verified

Apr 10, 2026
  🆕 community/redis-mcp v0.8.0 published
  ⚠️  legacy/old-server deprecated

Apr 8, 2026
  🆕 datadog/monitoring-agent v2.0.0 published
  🆕 datadog/logs-mcp v1.1.0 published
```

**Scope:** Requires an activity/event log in the backend (or can be derived from
`published_at` and `updated_at` timestamps on entries). Frontend renders a timeline
component. Could also be exposed as an RSS/Atom feed for subscriptions.

> **Decision: Accepted (derive from existing data)**
>
> Auto-derive from `published_at` and `updated_at` timestamps using the
> existing listing API with `sort=published_at_desc`. No separate event
> log needed for v1. Could also expose as RSS/Atom feed later.

---

## 11. Visual & Interaction Design

---

### 11.1 Icon system for resource types

**Problem:** MCP servers and agents use generic lucide icons (`Server` and `Bot`).
As the registry grows to include more resource types (skills, prompts, publishers),
a consistent icon language becomes important for quick visual scanning.

**What to do:** Define a clear icon set:
- MCP Server: plug/connector icon (represents connecting to external tools).
- Agent: robot/brain icon (represents autonomous action).
- Publisher: building/org icon (represents the organization).
- Skill: lightning/zap icon (represents a capability).
- Prompt: message/chat icon (represents a template).

Use these consistently in: navigation, cards, breadcrumbs, search results, admin
tables, and the home page.

**Example:**
```
Navigation:
  🔌 MCP Servers    🤖 Agents    🏢 Publishers

Search results:
  🔌 postgres-mcp        MCP Server
  🤖 code-review-agent   Agent
  🏢 Anthropic           Publisher
```

**Scope:** Choose icons from lucide-react (already a dependency). Create a shared
`ResourceIcon` component. Apply across all pages. Frontend-only.

> **Decision: Accepted**
>
>

---

### 11.2 Color-coded status badges

**Problem:** Status badges (draft, published, deprecated) may use the same visual
style or inconsistent colors across different pages. Admins and users need instant
recognition of entry status without reading the label.

**What to do:** Define and enforce a consistent color system:
- **Published** — green background, represents "live and available."
- **Draft** — yellow/amber background, represents "work in progress."
- **Deprecated** — red/muted background, represents "avoid using."

Apply everywhere: cards, detail pages, admin tables, admin dashboard.

**Example:**
```
[ Published ]    ← green-100 bg, green-800 text (light mode)
                   green-900 bg, green-200 text (dark mode)

[ Draft ]        ← amber-100 bg, amber-800 text
                   amber-900 bg, amber-200 text

[ Deprecated ]   ← red-100 bg, red-800 text
                   red-900 bg, red-200 text
```

**Scope:** Audit existing badge components, standardize color classes, and ensure dark
mode variants. Purely CSS/Tailwind changes.

> **Decision: Already consistent — closed**
>
> Audit result: badges are already consistent across the entire frontend.
> All status/visibility badges use centralized `statusVariant()` and
> `visibilityVariant()` helper functions from `badge.tsx`:
> - Published → green (`bg-green-100 text-green-800` / dark variants)
> - Deprecated → red (destructive variant)
> - Draft → gray (muted variant)
> - Public → primary color / Private → secondary color
> Dark mode variants are properly defined. No changes needed.

---

### 11.3 Card hover previews

**Problem:** On listing pages, users must click into a detail page to see capabilities
or skills. Then they click back, check the next card, click in again — lots of
back-and-forth navigation to compare entries.

**What to do:** On hover (desktop) or long-press (mobile), show an expanded preview
tooltip or popover with the entry's top capabilities/skills, install command, and
publisher info.

**Example:**
```
Hovering over "postgres-mcp" card:

┌─ postgres-mcp (expanded preview) ────────────────┐
│                                                   │
│ Tools: query, list_tables, describe, insert       │
│ Resources: schema://, data://                     │
│ Transport: stdio                                  │
│ Install: npx @anthropic/postgres-mcp         [📋] │
│                                                   │
│ by Anthropic ✓  |  Published Apr 5, 2026         │
│                                                   │
│ [View details →]                                 │
└──────────────────────────────────────────────────┘
```

**Scope:** A `HoverCard` component (shadcn/ui has one) with a richer data display.
May need to include more fields in the list API response, or lazy-fetch the detail
on hover. Frontend with possible minor API optimization.

> **Decision: Accepted**
>
>

---

### 11.4 Smooth page transitions

**Problem:** Navigating between pages causes a full re-render with no visual
continuity. The content flashes in, which feels abrupt — especially on the
list → detail → list flow that users repeat frequently.

**What to do:** Add subtle CSS transitions between routes. Options:
- Fade in/out (simplest, least disorienting).
- Slide from right on detail page open, slide back on return.
- Shared element transition (card → detail header) for a premium feel.

**Example:**
```
Listing page → click card → Detail page

Without transition:
  [listing] → [blank] → [detail]     (jarring flash)

With fade transition:
  [listing] → [listing fading] → [detail fading in]  (smooth)

With slide transition:
  [listing] ← [detail slides in from right]          (contextual)
```

**Scope:** Use React Router's `useNavigation` hook + CSS transitions, or a library
like `framer-motion`. Moderate effort to get right without janky intermediate states.

> **Decision: Deferred**
>
> Polish item. Defer until the core UX improvements are shipped. Can
> revisit with a prototype at that point.

---

### 11.5 Sticky header with context

**Problem:** On detail pages, when the user scrolls down past the title and metadata
to read capabilities, skills, or installation instructions, the entry name and key
actions disappear above the fold. They lose context of what entry they're looking at.

**What to do:** Add a compact sticky header that appears when the main title scrolls
out of view. It shows the entry name, version, status badge, and quick actions (copy
install command, view JSON link).

**Example:**
```
(scrolled down — sticky header appears)

┌──────────────────────────────────────────────────────┐
│ 🔌 postgres-mcp  v1.2.0  [Published]    [📋 Install] │
└──────────────────────────────────────────────────────┘

(rest of page content below)
│ Capabilities                                         │
│ ...                                                  │
```

**Scope:** An `IntersectionObserver` on the title element to toggle the sticky header.
A new `StickyDetailHeader` component. Frontend-only.

> **Decision: Accepted**
>
>

---

## Final Priority Ranking (post-review)

### Tier 1 — High impact, do first

1. **Global search in the hero** (1.2) — accepted
2. **One-click install / MCP config generator** (3.4, 8.3) — accepted, MCP hosts top priority
3. **Surface capabilities + skills prominently** (3.2, 4.1) — accepted
4. **Transport filter** (2.1) — accepted (stdio, SSE, Streamable HTTP only)
5. **Surface missing fields** (4.5) — accepted
6. **Responsive detail pages** (5.7) — accepted, mobile UX is very bad currently

### Tier 2 — Strong value, builds on Tier 1

7. **Tabbed detail layout** (3.1) — accepted, more fields in v2
8. **README / long description** (3.3) — accepted
9. **Copy-friendly identifiers everywhere** (8.2) — accepted
10. **Breadcrumb navigation** (5.1) — accepted
11. **Featured / popular entries on home** (1.1) — accepted
12. **Deep-linkable filters** (5.4) — accepted
13. **Category / tag cloud** (1.3) — accepted
14. **Copy install command on cards** (2.7) — accepted
15. **Loading skeletons + empty states** (5.2, 5.3) — accepted

### Tier 3 — Enrichment & trust

16. **Publisher public profile pages** (7.2) — accepted (project/teams in v2)
17. **Publisher card sidebar** (3.7) — accepted
18. **Namespace as clickable nav** (7.3) — accepted
19. **Version history** (3.5) — accepted
20. **Freshness indicator** (9.2) — accepted
21. **Verified badge on entries** (9.1) — accepted
22. **Unified explore page** (7.1) — accepted
23. **Sort options** (2.3) — accepted (basic now, popularity in v2)
24. **Ecosystem filter** (2.2) — accepted (only for stdio transport)
25. **Skill tags on agent cards** (2.5) — accepted
26. **Contextual tooltips** (10.3) — accepted
27. **"What is MCP/A2A?" explainers** (10.1) — accepted

### Tier 4 — Detail page enrichment

28. **Authentication guide for agents** (4.3) — accepted
29. **Input/Output modes explained** (4.4) — accepted
30. **Agent connection snippets** (8.4) — accepted
31. **Related servers** (3.6) — accepted
32. **Icon system** (11.1) — accepted
33. **Dark mode polish** (5.6) — accepted
34. **Sticky header with context** (11.5) — accepted
35. **Card hover previews** (11.3) — accepted
36. **Compatibility matrix** (9.3) — accepted

### Tier 5 — v2 / platform transition

37. **Admin dashboard with metrics** (6.1) — accepted (Tool Gateway metrics in v2)
38. **Inline status workflow** (6.2) — accepted
39. **Bulk actions** (6.3) — accepted
40. **Report an issue + admin queue** (8.6) — accepted (admin queue required)
41. **Diff between versions** (8.5) — accepted (auto-generated only)
42. **Health/uptime indicator** (9.4) — accepted (Tool Gateway in v2)
43. **Community signals** (9.5) — accepted (anonymous counters: view + copy)
44. **Getting Started guide** (10.2) — accepted (static page)
45. **Changelog / What's new** (10.4) — accepted (derive from timestamps)

### Closed / Rejected / Deferred

- ~~5.5 Keyboard shortcuts~~ — **Rejected** (over-engineered for a registry)
- ~~4.6 A2A Agent Card preview~~ — **Rejected** (JSON link is sufficient)
- ~~8.1 API playground~~ — **Already implemented** (Scalar at `/docs`)
- ~~11.2 Color-coded status badges~~ — **Already consistent** (no changes needed)
- ~~7.4 Sitemap / overview page~~ — **Deferred** (very low priority)
- ~~11.4 Smooth page transitions~~ — **Deferred** (polish, revisit later)
- ~~1.5 Publisher spotlight~~ — **Deferred** (publisher vs team/project TBD)
- ~~4.2 "Try it" / examples~~ — **Deferred** (auth/security challenges)
- ~~6.4 Publisher association on forms~~ — **Deferred** (accepted as v1, project/team in v2)
