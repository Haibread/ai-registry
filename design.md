# AI Registry — Design Document

This document covers the full design of the AI Registry: system architecture,
observability strategy, data and API design, and UI/UX specification.

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [Observability Design](#2-observability-design)
3. [Data & API Design](#3-data--api-design)
4. [UI/UX Design](#4-uiux-design)

---

## 1. System Architecture

### 1.1 Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          Clients                                │
│                                                                 │
│   Browser (Public UI)   Browser (Admin UI)   CI/CD (API key)   │
└────────────┬───────────────────┬──────────────────┬────────────┘
             │                   │                  │
             ▼                   ▼                  ▼
┌─────────────────────────────────────────────────────────────────┐
│            Static SPA (Vite + React Router v7)                  │
│  / · /mcp · /agents · /publishers           Public routes       │
│  /admin/*                                   Auth-guarded        │
│  — TanStack Query v5 against /api/v1/                           │
│  — oidc-client-ts (PKCE, no client secret) for /admin           │
│  — Served as static files by nginx; no server-side rendering    │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTP / JSON
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Go Backend (chi)                         │
│                                                                 │
│  Middleware chain:                                              │
│  OTel trace → request-id → CORS → rate-limit → auth guard       │
│  → publisher-role / reviewer guards (per route)                 │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐   │
│  │  /api/v1/    │  │  /v0/ (MCP)  │  │  /.well-known/     │   │
│  │  publishers/ │  │  servers     │  │  oauth-protected-  │   │
│  │  ↳ grants    │  │  publish     │  │  resource          │   │
│  │  groups/     │  └──────────────┘  │  agent-card.json   │   │
│  │  users/      │                    │  jwks.json         │   │
│  │  mcp/*       │                    └────────────────────┘   │
│  │  agents/*    │                                              │
│  │  review-     │                                              │
│  │   queue      │                                              │
│  │  audit       │                                              │
│  └──────────────┘                                              │
│                                                                 │
│  Internal packages:                                             │
│  domain │ store │ auth │ mcp │ agents │ bootstrap │ observ.    │
└──────────────────────┬──────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
┌──────────────────────┐  ┌──────────────────────┐
│     PostgreSQL       │  │   Keycloak (IdP)     │
│                      │  │                      │
│  publishers          │  │  OIDC / OAuth 2.1    │
│  users / groups      │  │  JWKS endpoint       │
│  role_grants         │  │  realm role: admin   │
│  mcp_servers         │  │  groups: membership  │
│  mcp_server_versions │  │    + the reviewer    │
│  agents              │  │    group             │
│  agent_versions      │  └──────────────────────┘
│  audit_log           │
│  reports             │
│                      │  ┌──────────────────────┐
└──────────────────────┘  │   OTel Collector     │
                          │                      │
                          │  OTLP gRPC :4317     │
                          │  → Jaeger (traces)   │
                          │  → Prometheus (metr) │
                          │  → Loki (logs)       │
                          └──────────────────────┘
```

### 1.2 Request Flows

**Public read (MCP server list)**
```
Browser → SPA (TanStack Query) → GET /api/v1/mcp/servers
  → OTel middleware (start span)
  → rate-limit check
  → handler: store.ListMCPServers(visibility=public)
    → Postgres query (child span)
  → JSON response
  → OTel middleware (end span, record latency metric)
```

**Publisher write (Editor submits a version for review)**
```
Admin SPA (oidc-client-ts or local session) → POST .../versions/{v}/submit
  → OTel middleware (start span)
  → auth middleware: validate JWT (issuer, signature, audience)
  → RequirePublisherRole(Editor) middleware:
       • resolve the target publisher from the {namespace} path segment
       • resolve the caller's effective role on that publisher from
         role_grants (direct user grant or via a group the JWT lists),
         falling back to claim groups when no users row is provisioned
       • allow if the effective role satisfies Editor OR Server Admin
  → handler: domain.TransitionToPendingReview(version, reason="submit")
    → Postgres UPDATE on review_state, increment revision (child span)
  → audit log write (child span)
  → 204 No Content
  → OTel middleware (end span, increment submit counter)
```

**Reviewer approval**
```
Admin SPA → POST /api/v1/review-queue / .../{kind}/{ns}/{slug}/versions/{v}/approve
  → auth middleware: validate JWT
  → RequireReviewer middleware: claim must include AUTH_REVIEWER_GROUP
    OR realm role "admin"
  → revision-mismatch check (discriminated 409 if stale)
  → handler: PublishMCPServerVersion(...) — publishes the version,
    flips review_state to none, stamps reviewed_by/at/decision
  → audit log write
  → 204 No Content
```

**A2A Agent Card**
```
MCP client / browser → GET /agents/{ns}/{slug}/.well-known/agent-card.json
  → handler: store.GetAgentWithLatestVersion()
  → agents.GenerateCard(agent, version) → AgentCard struct
  → JSON response (application/json)
```

### 1.3 Deployment Topology

**Development (docker-compose)**
```
postgres:5432
keycloak:8080
server:8081         ← go run / air hot-reload
web:3000            ← vite dev (HMR, proxies /api/* to server:8081)
otel-collector:4317
jaeger:16686
```

**Production (docker-compose prod profile)**
```
postgres (managed or container with volume)
server (multi-stage Docker image, distroless)
web (vite build output served by nginx; nginx proxies /api/* /v0/*
     /config.json to the server upstream)
reverse proxy (Caddy or nginx) → TLS termination
otel-collector → external Prometheus / Grafana / Tempo
```

**Kubernetes (Helm chart)**
```
Deployment: server (2+ replicas, HPA on CPU)
Deployment: web (2+ replicas)
Service + Ingress (with TLS via cert-manager)
PodDisruptionBudget on both
ExternalSecret → Postgres creds, OIDC client secret
ServiceMonitor → Prometheus scrape
```

---

## 2. Observability Design

### 2.1 Principles

- A single OTel SDK setup in `/internal/observability/` provides a `TracerProvider`,
  `MeterProvider`, and `LoggerProvider`. These are wired into `context.Context`
  at startup and never created ad-hoc.
- Every exported function that touches the network or DB receives a `context.Context`
  and propagates the span.
- Structured logs always carry `trace_id` and `span_id` to enable log-to-trace
  correlation in the collector pipeline.

### 2.2 Tracing

| Span name | Kind | Attributes |
|-----------|------|------------|
| `http.server` (per request) | SERVER | `http.method`, `http.route`, `http.status_code`, `http.request_content_length` |
| `db.query` (per SQL call) | CLIENT | `db.system=postgresql`, `db.operation`, `db.sql.table` |
| `mcp.publish` | INTERNAL | `mcp.server_id`, `mcp.version`, `publisher.slug` |
| `agent.card_generate` | INTERNAL | `agent.id`, `agent.version` |
| `auth.jwt_validate` | INTERNAL | `auth.method=jwt\|apikey`, result |

Propagation format: W3C TraceContext (`traceparent` / `tracestate` headers).

### 2.3 Metrics

All metrics are registered once in `/internal/observability/metrics.go`.

| Metric name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `registry.http.requests.total` | Counter | `method`, `route`, `status` | Total HTTP requests |
| `registry.http.request.duration` | Histogram | `method`, `route`, `status` | Latency in ms (buckets: 5, 25, 100, 250, 500, 1000, 5000) |
| `registry.mcp.servers.total` | UpDownCounter | `status`, `visibility` | Live count of MCP server entries |
| `registry.mcp.versions.published` | Counter | `publisher` | Versions published |
| `registry.agents.total` | UpDownCounter | `status`, `visibility` | Live count of agent entries |
| `registry.auth.failures` | Counter | `reason` (`invalid_token`, `expired`, `missing`, `forbidden`) | Auth failures |
| `registry.ratelimit.hits` | Counter | `route` | Rate-limit rejections |

### 2.4 Structured Logging

Log format: JSON, emitted via `slog` with an OTel bridge so records flow
through the `LoggerProvider` to the collector.

Required fields on every log line:

```json
{
  "time": "2026-04-07T12:00:00Z",
  "level": "INFO",
  "msg": "...",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "service.name": "ai-registry-server",
  "service.version": "0.1.0"
}
```

Log levels:
- `DEBUG`: SQL queries, cache decisions (disabled in prod by default).
- `INFO`: Request in/out, publish events, auth events.
- `WARN`: Rate-limit hits, validation errors, degraded dependencies.
- `ERROR`: Unhandled errors, DB failures, OTel export failures.

Never log: `Authorization` header value, raw JWT, API key plaintext.

### 2.5 Export Configuration (env)

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_SERVICE_NAME=ai-registry-server
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

---

## 3. Data & API Design

### 3.1 Entity-Relationship Diagram

```
publishers ──< mcp_servers ──< mcp_server_versions
           │
           └─< agents      ──< agent_versions

publishers ──< role_grants >── users / groups   (authorization, ADR 0006)
groups     ──< group_members >── users

audit_log (polymorphic: resource_type + resource_id, includes synthetic
           bootstrap-loader events)
reports (polymorphic: target_type + target_id; admin triages)
```

Every MCP server / agent belongs to exactly one publisher. The workspace
layer (ADR 0001/0002, migrations `000008`–`000011`) was removed by
migration `000013` (ADR 0006): `publisher_id` is restored `NOT NULL` on
resources, the `(publisher_id, slug)` unique key is back, and the
`workspaces` table is dropped. Authorization is now publisher-scoped
RBAC — `role_grants` ties a role (Viewer/Editor/Reviewer/Admin) to a user
or group on a publisher.

### 3.2 Key Table Schemas

```sql
-- publishers
id          TEXT PRIMARY KEY,          -- ULID
slug        TEXT UNIQUE NOT NULL,
name        TEXT NOT NULL,
contact     TEXT,
verified    BOOLEAN NOT NULL DEFAULT false,
created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()

-- role_grants (authorization, ADR 0006 / migration 000012)
id             TEXT PRIMARY KEY,       -- ULID
publisher_id   TEXT REFERENCES publishers(id),  -- NULL = global (server-wide) grant
principal_type TEXT NOT NULL,          -- 'user' | 'group'
principal_id   TEXT NOT NULL,          -- users.id or groups.id
role           TEXT NOT NULL,          -- viewer | editor | reviewer | admin
source         TEXT NOT NULL,          -- 'api' | 'seed'
created_at     TIMESTAMPTZ NOT NULL DEFAULT now()

-- mcp_servers
id           TEXT PRIMARY KEY,         -- ULID
publisher_id TEXT NOT NULL REFERENCES publishers(id),
slug         TEXT NOT NULL,
name         TEXT NOT NULL,
description  TEXT,
homepage_url TEXT,
repo_url     TEXT,
license      TEXT,
visibility   TEXT NOT NULL DEFAULT 'private',  -- private | public
status       TEXT NOT NULL DEFAULT 'draft',    -- draft | published | deprecated
created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
UNIQUE (publisher_id, slug)
-- Slug uniqueness is publisher-scoped: two different publishers can each
-- own a `files` server, but a publisher cannot own two.

-- mcp_server_versions
id                  TEXT PRIMARY KEY,  -- ULID
server_id           TEXT NOT NULL REFERENCES mcp_servers(id),
version             TEXT NOT NULL,     -- semver
runtime             TEXT NOT NULL,     -- stdio | http | sse | streamable_http
install             JSONB NOT NULL,
capabilities        JSONB NOT NULL,
tools               JSONB NOT NULL DEFAULT '[]',
protocol_version    TEXT NOT NULL,
published_at        TIMESTAMPTZ,       -- NULL until published

-- Change-approval (migration 000010) — orthogonal to status/published_at
review_state        TEXT NOT NULL DEFAULT 'none',
                                       -- none | pending_review | rejected
revision            INTEGER NOT NULL DEFAULT 0,
                                       -- monotonic; bumped on every edit/transition
submitted_by        TEXT,              -- OIDC sub of the submitter
submitted_by_email  TEXT,
submitted_at        TIMESTAMPTZ,
reviewed_by         TEXT,
reviewed_by_email   TEXT,
reviewed_at         TIMESTAMPTZ,
review_decision     TEXT,              -- approved | rejected | NULL
rejection_reason    TEXT,

released_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
UNIQUE (server_id, version)

-- agents / agent_versions: symmetric to mcp_servers / mcp_server_versions
-- including the same change-approval column set on agent_versions.

-- audit_log
id            BIGSERIAL PRIMARY KEY,
actor_subject TEXT NOT NULL,           -- OIDC sub or "system:bootstrap"
actor_email   TEXT,
action        TEXT NOT NULL,           -- e.g. mcp_server.publish, publisher.created
resource_type TEXT NOT NULL,
resource_id   TEXT NOT NULL,
resource_ns   TEXT,                    -- publisher slug for scoped resources
resource_slug TEXT,
metadata      JSONB,
created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

Indexes: `(publisher_id, slug)` on entry tables, `(server_id, version)` on
versions, `(publisher_id, principal_type, principal_id, role)` on
`role_grants`, `status`, `visibility`,
`review_state` (partial index `WHERE review_state = 'pending_review'`)
to make the review queue scan cheap. Full-text index on
`name || ' ' || description` via `tsvector`.

### 3.3 Version Lifecycle State Machine

Two orthogonal axes: the publish-status axis (`status` / `published_at`)
and the review-state axis (`review_state`, introduced by migration
`000010`). The publish axis still drives what's visible to public
readers; the review axis decides who is allowed to make the next
publish-axis transition.

**Publish axis** (unchanged from v0.2):

```
         ┌─────────┐
         │  draft  │  ← created by POST /versions
         └────┬────┘
              │ :publish (admin or reviewer-approved)
              ▼
       ┌────────────┐
       │ published  │  ← immutable; metadata edits forbidden
       └─────┬──────┘
             │ :deprecate
             ▼
      ┌────────────┐
      │ deprecated │  ← still readable; hidden from default listing
      └────────────┘
```

Published versions are immutable: no `PATCH` on a `mcp_server_versions`
row after `published_at` is set.

**Review axis** (change-approval workflow, ADR 0003):

```
                    edit / create
                          │
                          ▼
                     ┌─────────┐
              ┌──────│  none   │──────┐
              │      └────┬────┘      │
              │           │ :submit   │
       :reject│           │           │ :approve
              │           ▼           │  (reviewer)
              │     ┌──────────────┐  │
              │     │pending_review│  │
              │     └──────┬───────┘  │
              │            │ :withdraw│
              │            │ (author) │
              ▼            │          ▼
        ┌──────────┐       │     publish-axis
        │ rejected │ ──────┘     transition runs
        └────┬─────┘   :resubmit (publish handler)
             │
             │  edit (revision++)
             ▼
        ┌──────────┐
        │   none   │  ← rejected versions are still drafts; another submit
        └──────────┘    moves them back to pending_review.
```

Publisher Editors can drive `none → pending_review` and
`pending_review → none` (withdraw). Only Reviewers can drive
`pending_review → none` via approve (which also runs the publish-axis
transition) or `pending_review → rejected`. Every transition increments
the row's `revision` counter, so concurrent edits surface a discriminated
409 (`review-revision-mismatch`) instead of clobbering each other.

Entries themselves can also have a pending-deletion review attached;
this is implemented as a row in the same review queue scoped to the
entry rather than a single version.

### 3.4 Pagination & Filtering

All list endpoints use **cursor-based pagination** (opaque base64 cursor encoding
`(created_at, id)` for stable ordering):

```
GET /api/v1/mcp/servers?q=search&namespace=acme&limit=20&cursor=<opaque>

Response:
{
  "items": [...],
  "next_cursor": "<opaque>",   // absent if last page
  "total_count": 142           // approximate, from stats table
}
```

### 3.5 Error Catalogue (RFC 7807)

```json
{
  "type": "https://registry.example.com/errors/not-found",
  "title": "Resource not found",
  "status": 404,
  "detail": "MCP server 'acme/my-server' does not exist.",
  "instance": "/api/v1/mcp/servers/acme/my-server"
}
```

| Type slug | Status | Meaning |
|-----------|--------|---------|
| `not-found` | 404 | Entity does not exist or is not visible |
| `forbidden` | 403 | Authenticated but lacks the required role on the target publisher (Editor/Reviewer/Admin), Server Admin, or the reviewer group (depending on the route) |
| `unauthorized` | 401 | Missing or invalid bearer token |
| `validation-error` | 422 | Request body failed schema validation; `errors[]` extension field |
| `conflict` | 409 | Duplicate slug or version |
| `immutable` | 409 | Attempt to mutate a published (immutable) version |
| `review-state-mismatch` | 409 | Transition not allowed from the current `review_state` (e.g. approve called on a `none` row) |
| `review-revision-mismatch` | 409 | Caller's `revision` doesn't match the row — the version was edited under them; refresh and retry |
| `review-already-pending` | 409 | Another version on the same entry is already pending review (one-at-a-time invariant) |
| `already-published` | 409 | Approve called on an already-published version |
| `rate-limited` | 429 | Too many requests; `Retry-After` header set |
| `internal` | 500 | Unexpected server error |

The `review-*` and `already-published` types are **discriminated** — the
admin UI maps each `type` to a friendly inline message ("the version
was edited since this page loaded — refresh"; "another version on this
entry is already pending review"; etc.). Don't fold them into a generic
`conflict`.

---

## 4. UI/UX Design

### 4.1 Design System

**Framework**: Vite + React 19 + React Router v7 + shadcn/ui + Tailwind CSS v4. The whole application ships as a static SPA; nginx serves the bundle and proxies API paths to the Go backend.

#### Color Palette

| Token | Tailwind / HSL | Usage |
|-------|---------------|-------|
| `background` | `slate-50` / `#f8fafc` | Page background (light) |
| `foreground` | `slate-900` / `#0f172a` | Body text |
| `primary` | `indigo-600` / `#4f46e5` | CTA buttons, active nav, links |
| `primary-foreground` | `white` | Text on primary |
| `secondary` | `slate-100` | Secondary buttons, tag backgrounds |
| `muted` | `slate-200` | Dividers, disabled states |
| `muted-foreground` | `slate-500` | Placeholder text, captions |
| `accent` | `indigo-50` | Hover states, card hover ring |
| `destructive` | `red-600` | Delete actions, error states |
| `success` | `emerald-600` | Published badge, success toasts |
| `warning` | `amber-500` | Deprecated badge, warning banners |
| `border` | `slate-200` | Card and input borders |
| `card` | `white` | Card background |

Dark mode mirrors the same tokens with `slate-950` background and `slate-100`
foreground, toggled via a `class="dark"` on `<html>`. shadcn/ui's CSS variable
system handles the swap automatically.

#### Typography

| Role | Font | Weight | Size |
|------|------|--------|------|
| Display heading | Tailwind `font-sans` (system stack) | 700 | `text-3xl` – `text-5xl` |
| Section heading | `font-sans` | 600 | `text-xl` – `text-2xl` |
| Body | `font-sans` | 400 | `text-sm` – `text-base` |
| Label / caption | `font-sans` | 500 | `text-xs` – `text-sm` |
| Code / version | `font-mono` (system monospace stack) | 400 | `text-xs` – `text-sm` |

The web app does not bundle a webfont — Tailwind's default system stacks
(`font-sans` → `ui-sans-serif, system-ui, …`; `font-mono` →
`ui-monospace, SFMono-Regular, …`) keep first-paint fast and the bundle
small. The original Next.js scaffold used `Geist` via `next/font`; that
loader was dropped along with the rest of the Next.js stack in Phase 6
(see [ADR 0004](docs/adr/0004-vite-spa-migration.md)).

#### Spacing & Radius

- Base unit: `4px` (Tailwind default).
- Card radius: `rounded-xl` (12px).
- Button radius: `rounded-lg` (8px).
- Input radius: `rounded-md` (6px).
- Page max-width: `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8`.

---

### 4.2 Public UI Layout

```
┌─────────────────────────────────────────────────────┐
│  TOPBAR (sticky, white, border-b)                   │
│  [Logo]  MCP Servers  Agents  Docs     [Search ⌘K]  │
└─────────────────────────────────────────────────────┘
│                                                     │
│  PAGE CONTENT (max-w-7xl)                           │
│                                                     │
└─────────────────────────────────────────────────────┘
│  FOOTER (slate-900 bg)                              │
│  Links · Status · GitHub · Docs                     │
└─────────────────────────────────────────────────────┘
```

**Top bar** (`h-16`, `sticky top-0 z-50`):
- Left: logo mark (indigo SVG) + "AI Registry" wordmark.
- Center: `<nav>` links — MCP Servers, Agents, Docs.
- Right: command-palette trigger (`⌘K`), dark-mode toggle, "Admin →" link (only
  if session exists).

**Homepage** (`/`):
- Hero section: headline + sub-headline + search bar (prominent, centered).
- Two stat tiles: "N MCP Servers" / "N Agents" (from `/api/v1/stats`).
- Featured entries grid (6 cards, pinned by admin).

**Listing pages** (`/mcp`, `/agents`):
- Left sidebar (240px, `lg:block hidden`): filter panel — status, runtime
  (MCP only), publisher, protocol version. Checkboxes, applied as query params.
- Main: search input + sort dropdown + card grid (3 cols desktop, 2 tablet, 1
  mobile).
- Card anatomy:
  ```
  ┌───────────────────────────────┐
  │ [Icon 40px]  Name             │
  │              namespace/slug   │
  │                               │
  │ Description (2-line clamp)    │
  │                               │
  │ [runtime badge] [version tag] │
  │ ★ publisher · updated N days  │
  └───────────────────────────────┘
  ```
- Pagination: "Load more" button (appends to list), not page numbers.

**Detail pages** (`/mcp/[ns]/[slug]`, `/agents/[ns]/[slug]`):
- Two-column: main (content) 2/3 + aside (metadata) 1/3.
- Tabs: Overview · Versions · Install.
- Install tab shows copy-ready shell snippets per runtime/package manager.
- Versions tab: table with semver, release date, protocol version, status badge.

---

### 4.3 Admin UI Layout

Guarded by `<RequireAuth>` wrapping every `/admin/*` route. Authentication
runs entirely client-side via `oidc-client-ts` (PKCE public client; no
Next.js, no Auth.js, no client secret). Unauthenticated visits trigger a
redirect to the IdP authorize endpoint and a callback flow handled by
`/auth/callback`.

```
┌──────────┬──────────────────────────────────────────┐
│ SIDEBAR  │  TOPBAR (breadcrumb + theme + user menu)  │
│ md+ only │──────────────────────────────────────────│
│          │                                           │
│ Dashboard│  PAGE CONTENT                             │
│ Review   │                                           │
│   queue  │  ⓘ                                        │
│ Publishers│                                          │
│ Groups   │                                           │
│ Users    │                                           │
│ MCP      │                                           │
│  Servers │                                           │
│ Agents   │                                           │
│ Reports  │                                           │
│ Audit    │                                           │
│ API Keys │ (placeholder — see PLAN.md v0.4.x)        │
│          │                                           │
└──────────┴──────────────────────────────────────────┘
```

**Sidebar** (`w-56`, `border-r bg-muted/30`, `hidden md:block`):
- Each nav item: icon (lucide-react) + label, active style via `cn()`.
- The Review queue item carries a live count badge fed by a TanStack
  Query hook against `/api/v1/review-queue?limit=99` with a 30-second
  refetch interval (reads "99+" past 99). The cache is invalidated on
  every change-approval mutation toast so the count stays current.
- Mobile (`<md`): the static sidebar is hidden. A hamburger button in
  the header opens a fixed-position drawer that reuses the same
  `AdminSidebar` component with `mobile={true}`. The drawer dismisses
  on Escape, on backdrop click, on a nav-link tap, and on
  `location.pathname` change. Body scroll is locked while open.

**Grants section** (publisher detail page):
- Renders below the publisher Edit / Delete actions, before the MCP
  servers and agents tables.
- Lists the publisher's role grants — principal (user/group) · role ·
  source — with a "Grant role" form to add a Viewer/Editor/Reviewer/Admin
  grant to a user or group and a per-row Revoke action. Backed by the
  `/api/v1/publishers/{slug}/grants` endpoints (ADR 0006).

**Review queue page** (`/admin/review`):
- Reviewer-only (gated by `RequireReviewer`; non-reviewers see a 403
  page).
- One list of pending items, each rendered as either a "version
  pending review" card (entry slug, version, revision, submitter
  email + timestamp) or a "deletion request" card.
- Actions: Approve · Reject (which opens an inline reason form;
  reason is required and stored on the version's
  `rejection_reason`).
- Per-version cards on entry detail pages mirror the same data with
  Submit / Withdraw / Resubmit buttons gated by `review_state`.

**Data tables** (shadcn/ui `<DataTable>` with TanStack Table):
- Column sorting, row selection checkboxes for bulk actions.
- Inline action menu (ellipsis `⋯`): Edit, Publish, Deprecate, Delete.
- Status and visibility shown as colored badges.
- Search/filter bar above the table.

**Forms**: native HTML forms + shadcn/ui `<Input>` / `<Label>` / `<Button>`.
No react-hook-form / zod dependency in the admin tree — forms are simple
enough that `FormData` parsing inside the submit handler suffices.
- Inline validation errors below each form, scoped to the action that
  produced them (`createError` / `editError` / `deleteError` rather
  than a single section banner). Every error region carries
  `role="alert"` so screen readers announce it.
- "Save changes" (primary, right) + "Cancel" (outline, left) on edit
  forms; reordering matches dialog conventions.
- Destructive actions use a `window.confirm` gate. The DeleteButton
  itself is rendered with quiet styling (outline + destructive text +
  faded border, fills red on hover) so it doesn't drown out the
  row's primary actions; the `confirm` dialog is the real safety
  net.

**Toast notifications** (`sonner`, mounted at the app root):
- Triggered on every change-approval mutation (submit, withdraw,
  approve, reject, request deletion) and on grant/group/user CRUD.
- Position: top-right; rich colors; close button.
- The cache for the sidebar's review-queue badge is invalidated
  alongside change-approval toasts so the count stays in sync.

---

### 4.4 Component Inventory

| Component | Location | Notes |
|-----------|----------|-------|
| `MCPCard` / `AgentCard` | `components/{mcp,agents}/*.tsx` | Used in all listing grids |
| `StatusBadge` / `VisibilityBadge` | `components/ui/badge.tsx` | draft/published/deprecated · private/public |
| `FilterBar` | `components/ui/filter-bar.tsx` | Search + namespace + status + visibility filters; debounced URL writes |
| `Table` (shadcn) | `components/ui/table.tsx` | Wraps responsive horizontal scroll; columns hide via Tailwind breakpoints |
| `InstallCommand` / `ConfigGenerator` | `components/{ui,mcp}/*.tsx` | Code blocks + copy |
| `AdminSidebar` | `components/layout/admin-sidebar.tsx` | Includes the live review-queue badge hook |
| `GrantsSection` | `components/admin/grants-section.tsx` | Lists/creates/revokes publisher role grants |
| `VersionsSection` | `components/admin/versions-section.tsx` | Per-version submit / withdraw / resubmit |
| `RequestDeletionButton` | `components/admin/request-deletion-button.tsx` | Submits a deletion review |
| `DeleteButton` | `components/admin/delete-button.tsx` | Quiet outline + window.confirm gate |
| `LifecycleStepper` | `components/admin/lifecycle-stepper.tsx` | Visual indicator of publish-axis state |
| `ReviewQueue` | `pages/admin/review/index.tsx` | Reviewer-only Approve / Reject UI |

---

### 4.5 Responsive Breakpoints

| Breakpoint | Width | Layout changes |
|------------|-------|----------------|
| `sm` | 640px | Search bar expands |
| `md` | 768px | 2-col card grid |
| `lg` | 1024px | 3-col grid; filter sidebar visible; admin sidebar visible |
| `xl` | 1280px | Detail page 2-col layout |

Mobile-first: all layouts start single-column and expand at breakpoints.

---

### 4.6 Accessibility

- Color contrast: all text/background pairs meet WCAG AA (4.5:1 normal, 3:1 large).
- Focus rings: `focus-visible:ring-2 ring-indigo-500` on all interactive elements.
- Semantic HTML: `<nav>`, `<main>`, `<aside>`, `<header>`, `<footer>` landmarks.
- ARIA labels on icon-only buttons; `aria-current="page"` on active nav links.
- Keyboard navigable command palette and dropdown menus.
