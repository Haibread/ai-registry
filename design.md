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
│                        Next.js App                              │
│  /app/(public)/*          /app/admin/*                         │
│  — SSR + RSC              — Auth.js (OIDC session)             │
│  — Generated TS client    — Same TS client (bearer token)      │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTP / JSON
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Go Backend (chi)                         │
│                                                                 │
│  Middleware chain:                                              │
│  OTel trace → request-id → CORS → rate-limit → auth guard      │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐   │
│  │  /api/v1/    │  │  /v0/ (MCP)  │  │  /.well-known/     │   │
│  │  mcp/*       │  │  servers     │  │  oauth-protected-  │   │
│  │  agents/*    │  │  publish     │  │  resource          │   │
│  │  publishers/ │  └──────────────┘  │  agent-card.json   │   │
│  │  users/      │                    └────────────────────┘   │
│  │  api-keys/   │                                              │
│  └──────────────┘                                              │
│                                                                 │
│  Internal packages:                                             │
│  domain │ store │ auth │ mcp │ agents │ observability           │
└──────────────────────┬──────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
┌──────────────────┐      ┌──────────────────────┐
│   PostgreSQL     │      │   Keycloak (IdP)      │
│                  │      │                      │
│  publishers      │      │  OIDC / OAuth 2.1    │
│  users           │      │  JWKS endpoint       │
│  mcp_servers     │      │  registry:admin role │
│  mcp_versions    │      └──────────────────────┘
│  agents          │
│  agent_versions  │      ┌──────────────────────┐
│  api_keys        │      │   OTel Collector      │
│  audit_log       │      │                      │
└──────────────────┘      │  OTLP gRPC :4317     │
                          │  → Jaeger (traces)   │
                          │  → Prometheus (metr) │
                          │  → Loki (logs)       │
                          └──────────────────────┘
```

### 1.2 Request Flows

**Public read (MCP server list)**
```
Browser → Next.js RSC → GET /api/v1/mcp/servers
  → OTel middleware (start span)
  → rate-limit check
  → handler: store.ListMCPServers(visibility=public)
    → Postgres query (child span)
  → JSON response
  → OTel middleware (end span, record latency metric)
```

**Admin write (publish new version)**
```
Admin UI → Auth.js session (access token) → POST /api/v1/mcp/servers/{ns}/{slug}/versions
  → OTel middleware (start span)
  → auth middleware: validate JWT → check registry:admin scope
    OR API-key middleware: hash lookup in api_keys table
  → admin guard (403 if not admin)
  → handler: validate payload → store.CreateMCPVersion()
    → Postgres INSERT (child span)
  → audit log write (child span)
  → 201 Created
  → OTel middleware (end span, increment publish counter)
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
backend:8081        ← go run / air hot-reload
web:3000            ← next dev
otel-collector:4317
jaeger:16686
```

**Production (docker-compose prod profile)**
```
postgres (managed or container with volume)
backend (multi-stage Docker image, distroless)
web (Next.js standalone output)
reverse proxy (Caddy or nginx) → TLS termination
otel-collector → external Prometheus / Grafana / Tempo
```

**Kubernetes (Helm chart)**
```
Deployment: backend (2+ replicas, HPA on CPU)
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
  "service.name": "ai-registry-backend",
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
OTEL_SERVICE_NAME=ai-registry-backend
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

---

## 3. Data & API Design

### 3.1 Entity-Relationship Diagram

```
publishers ──< mcp_servers ──< mcp_server_versions
           │
           └──< agents ──< agent_versions

users (from OIDC — cached/synced)
api_keys ──> publishers  (scoped per publisher)
audit_log (polymorphic: resource_type + resource_id)
```

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

-- mcp_server_versions
id               TEXT PRIMARY KEY,     -- ULID
server_id        TEXT NOT NULL REFERENCES mcp_servers(id),
version          TEXT NOT NULL,        -- semver
runtime          TEXT NOT NULL,        -- stdio | http | sse | streamable_http
install          JSONB NOT NULL,
capabilities     JSONB NOT NULL,
protocol_version TEXT NOT NULL,
checksum         TEXT,
signature        TEXT,
published_at     TIMESTAMPTZ,          -- NULL until published
released_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
UNIQUE (server_id, version)

-- agents / agent_versions: symmetric to mcp_servers / mcp_server_versions

-- api_keys
id           TEXT PRIMARY KEY,
publisher_id TEXT NOT NULL REFERENCES publishers(id),
key_hash     TEXT NOT NULL UNIQUE,     -- bcrypt hash
prefix       TEXT NOT NULL,            -- first 8 chars for display (apikey_XXXXXXXX...)
description  TEXT,
last_used_at TIMESTAMPTZ,
expires_at   TIMESTAMPTZ,
created_at   TIMESTAMPTZ NOT NULL DEFAULT now()

-- audit_log
id            BIGSERIAL PRIMARY KEY,
actor_subject TEXT NOT NULL,           -- OIDC sub or "apikey:<id>"
action        TEXT NOT NULL,           -- e.g. mcp_server.publish
resource_type TEXT NOT NULL,
resource_id   TEXT NOT NULL,
payload       JSONB,
created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

Indexes: `(publisher_id, slug)`, `(server_id, version)`, `status`, `visibility`,
full-text index on `name || ' ' || description` via `tsvector`.

### 3.3 Version Lifecycle State Machine

```
         ┌─────────┐
         │  draft  │  ← created by POST /versions
         └────┬────┘
              │ :publish
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

State transitions are admin-only. Published versions are immutable: no `PATCH`
on a `mcp_server_versions` row after `published_at` is set.

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
| `forbidden` | 403 | Authenticated but lacks `registry:admin` scope |
| `unauthorized` | 401 | Missing or invalid bearer token |
| `validation-error` | 422 | Request body failed schema validation; `errors[]` extension field |
| `conflict` | 409 | Duplicate slug or version |
| `immutable` | 409 | Attempt to mutate a published (immutable) version |
| `rate-limited` | 429 | Too many requests; `Retry-After` header set |
| `internal` | 500 | Unexpected server error |

---

## 4. UI/UX Design

### 4.1 Design System

**Framework**: Next.js 15 App Router + shadcn/ui + Tailwind CSS v4.

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
| Display heading | Geist (next/font) | 700 | `text-3xl` – `text-5xl` |
| Section heading | Geist | 600 | `text-xl` – `text-2xl` |
| Body | Geist | 400 | `text-sm` – `text-base` |
| Label / caption | Geist | 500 | `text-xs` – `text-sm` |
| Code / version | Geist Mono | 400 | `text-xs` – `text-sm` |

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

Guarded by Auth.js: unauthenticated requests redirect to the IdP login page.

```
┌──────────┬──────────────────────────────────────────┐
│ SIDEBAR  │  TOPBAR (breadcrumb + user menu)          │
│ 240px    │──────────────────────────────────────────│
│          │                                           │
│ Overview │  PAGE CONTENT                             │
│          │                                           │
│ MCP      │                                           │
│  Servers │                                           │
│  Publish │                                           │
│          │                                           │
│ Agents   │                                           │
│          │                                           │
│ Publishers│                                          │
│ API Keys │                                           │
│ Users    │                                           │
│ Audit Log│                                           │
│          │                                           │
│ [Avatar] │                                           │
│ Sign out │                                           │
└──────────┴──────────────────────────────────────────┘
```

**Sidebar** (`w-60`, `bg-slate-900 text-slate-100`):
- Logo + "Admin" badge at top.
- Nav groups with icons (lucide-react): each group collapsible.
- Active item: `bg-indigo-600 text-white rounded-md`.
- Bottom: avatar, name, email, sign-out.
- Mobile: hidden by default, slide-in drawer triggered by hamburger.

**Data tables** (shadcn/ui `<DataTable>` with TanStack Table):
- Column sorting, row selection checkboxes for bulk actions.
- Inline action menu (ellipsis `⋯`): Edit, Publish, Deprecate, Delete.
- Status and visibility shown as colored badges.
- Search/filter bar above the table.

**Forms** (shadcn/ui `<Form>` + react-hook-form + zod):
- Side-by-side layout on desktop (label left, input right in 2-col grid).
- Inline validation errors below each field.
- "Save draft" (secondary) + "Publish" (primary) button pair on version forms.
- Destructive actions (Delete, Deprecate) require a confirmation dialog with
  typed name confirmation for irreversible operations.

**Toast notifications** (shadcn/ui `<Sonner>`):
- Success: emerald border, "Published successfully."
- Error: red border, error message from `problem+json` detail field.
- Position: bottom-right.

---

### 4.4 Component Inventory

| Component | Location | Notes |
|-----------|----------|-------|
| `RegistryCard` | `components/registry/card.tsx` | Used in all listing grids |
| `StatusBadge` | `components/ui/status-badge.tsx` | draft/published/deprecated |
| `VisibilityBadge` | `components/ui/visibility-badge.tsx` | private/public |
| `RuntimeBadge` | `components/ui/runtime-badge.tsx` | stdio/http/sse/streamable_http |
| `DataTable` | `components/data-table/` | Generic, typed with TanStack |
| `CommandPalette` | `components/command-palette.tsx` | `⌘K` global search |
| `InstallSnippet` | `components/registry/install-snippet.tsx` | Code block + copy |
| `AgentCardViewer` | `components/agents/card-viewer.tsx` | Renders A2A card fields |
| `ConfirmDialog` | `components/ui/confirm-dialog.tsx` | Typed-name confirmation |
| `AdminSidebar` | `components/admin/sidebar.tsx` | Collapsible nav groups |

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
