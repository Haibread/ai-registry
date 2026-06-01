# UI/UX Implementation Plan — Remaining Work

> The original 10-batch plan is **done**. All shared primitives, listing
> filters/sort, detail-page restructure, home page, explore page, config/snippet
> generators, publisher pages, trust signals, detail enrichment, and the admin
> sweep (dashboard, lifecycle stepper, bulk actions, reports queue, view/copy
> counts, changelog, getting-started) shipped across v0.2.0–v0.3.x. See
> `CHANGELOG.md` for the landed work and `../PLAN.md` for the broader roadmap.
>
> What's left from the UI/UX catalog is small and mostly optional. The matching
> ideas live in [`ui-ux-proposals.md`](ui-ux-proposals.md).

---

## Remaining items

### Card hover previews — frontend only

`HoverCard` (shadcn/ui) on listing cards showing top capabilities/skills,
install command (CopyButton), and publisher info. Lazy-fetch detail on hover,
or widen the list response if a single round-trip is preferred.

- Files: a new `web/src/components/shared/card-hover-preview.tsx`, wired into
  `web/src/components/mcp/server-card.tsx` and
  `web/src/components/agents/agent-card.tsx`.
- Backend/DB: none (unless choosing to widen the list response).

### List view toggle (grid vs. table) — frontend only, pending mockup

Compact table view as an alternative to the card grid, persisted in
localStorage. Build only after validating with a prototype.

- Files: new `ServerTable` / `AgentTable` components, a toggle in the list
  pages, no API change.

### Health / uptime indicator — deferred to v2 (Tool Gateway)

Status dot for remote endpoints. Expected to come from the Tool Gateway in v2
rather than a standalone health-check worker, so no batch is scheduled now. If
built standalone it needs a periodic worker, an `endpoint_health` table, and a
new API field.

### Smooth page transitions — deferred polish

Fade/slide on route changes via React Router `useNavigation` + CSS or
`framer-motion`. Revisit after a prototype.

### Sitemap / overview page — deferred, very low priority

`/overview` listing namespaces with entry counts. Revisit only on SEO need or
significant growth.

---

## Summary

| Item | Scope | Backend | DB | Status |
|------|-------|---------|----|--------|
| Card hover previews | S | No | No | Accepted, not built |
| List view toggle | S | No | No | Pending mockup |
| Health / uptime | L | Yes | Yes | Deferred to v2 |
| Page transitions | S | No | No | Deferred (polish) |
| Sitemap / overview | S | Minor | No | Deferred (low priority) |
