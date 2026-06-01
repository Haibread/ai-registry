# UI/UX Improvement Proposals — Remaining Backlog

> Most of the original proposal catalog shipped across v0.2.0–v0.3.x (search,
> featured/tag home page, explore page, filters/sort, detail-page tabs,
> capabilities/skills, config + snippet generators, publisher pages, verified
> badges, freshness, version history/diff, README rendering, admin dashboard,
> lifecycle stepper, bulk actions, reports queue, view/copy counts, changelog,
> getting-started, and the shared component library). See `CHANGELOG.md` for
> what landed.
>
> This file now tracks only the handful of accepted-but-unbuilt ideas plus a
> couple of deliberately deferred ones. Verdicts use a **Decision** field:
> `accepted`, `deferred`, or `rejected`.

---

## 1. Card hover previews (was 11.3)

On listing pages, comparing entries means clicking into each detail page and
back. A `HoverCard` (shadcn/ui) on each card could show top capabilities/skills,
the install command (with copy), and publisher info without navigation —
lazy-fetching detail data on hover.

**Why it matters:** speeds up side-by-side comparison, the most common
power-user flow on a registry.

**Scope:** frontend `HoverCard` over the existing card components; may need to
lazy-fetch detail on hover or widen the list response.

> **Decision: Accepted** (not yet built)

---

## 2. List view toggle — grid vs. compact table (was 2.4)

The card grid is good for browsing but inefficient for scanning 50+ entries. A
toggle to a compact table (Name, Namespace, Version, Status,
Transport/Ecosystem, Updated — one row each) would help power users. Persist
the choice in localStorage; no API change.

**Why it matters:** dense scanning for maintainers and heavy users.

> **Decision: Needs a concrete mockup before building** — judge usefulness from
> a prototype first.

---

## 3. Smooth page transitions (was 11.4)

Route changes flash in with no continuity, notably on the repeated
list → detail → list flow. A subtle fade (or slide on detail open) would smooth
it. Use React Router's `useNavigation` + CSS transitions, or `framer-motion`.

**Why it matters:** perceived polish; low functional value.

> **Decision: Deferred** — polish item; revisit with a prototype once core UX
> work is settled.

---

## 4. Health / uptime indicator for remote endpoints (was 9.4)

Remote MCP servers and agents can go down silently. A periodic backend health
check could record status and surface a green/yellow/red dot on cards and
detail pages.

**Why it matters:** trust signal for remote entries — but heavy (worker +
status-history table + API field).

> **Decision: Deferred to v2** — likely sourced from the Tool Gateway (which
> will route every MCP/agent call) rather than a standalone health worker.

---

## 5. Sitemap / registry overview page (was 7.4)

A `/overview` page listing all namespaces with entry counts, grouped
alphabetically, each linking to its filtered listing. Also helps SEO indexing.

**Why it matters:** bird's-eye view; only valuable at larger scale.

> **Decision: Deferred (very low priority)** — revisit only if SEO becomes a
> concern or the registry grows to hundreds of namespaces.

---

## Rejected

- **Keyboard shortcuts** (was 5.5) — over-engineered for a registry. If
  anything, only a one-line `/`-to-focus-search shortcut is worth considering;
  no full shortcut system.
- **A2A Agent Card inline preview** (was 4.6) — the existing JSON
  download/link is sufficient.
