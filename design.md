# AI Registry — Design Document

Design intent and rationale for the AI Registry: system architecture, the auth
model, observability principles, the data lifecycle, and the non-obvious UI/UX
decisions. This document deliberately points at code for anything mechanical
(schemas, routes, tokens) and keeps only what an agent cannot quickly recover by
reading the repository.

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [Auth Model](#2-auth-model)
3. [Observability Design](#3-observability-design)
4. [Data Lifecycle](#4-data-lifecycle)
5. [UI/UX Design](#5-uiux-design)

---

## 1. System Architecture

### 1.1 Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          Clients                                │
│      Browser (Public UI)            Browser (Admin UI)         │
└────────────────────┬──────────────────────┬────────────────────┘
                     │                      │
                     ▼                      ▼
┌─────────────────────────────────────────────────────────────────┐
│            Static SPA (Vite + React Router v7)                  │
│  Public routes (browse) + /admin/* (auth-guarded)              │
│  TanStack Query against /api/v1/; the SPA is NOT an OIDC        │
│  client — it learns identity + grants from GET /api/v1/me.      │
│  Served as static files by nginx; no server-side rendering.     │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTP / JSON
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Go Backend (chi)                         │
│  Middleware: trace → request-id → CORS → rate-limit →           │
│              session auth → publisher-role / reviewer guard     │
│  Internal packages:                                             │
│  domain │ store │ auth │ mcp │ agents │ bootstrap │ observ.    │
└──────────────────────┬──────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          ▼                         ▼
┌──────────────────────┐  ┌──────────────────────┐
│     PostgreSQL       │  │   Keycloak (IdP)     │
│  registry state,     │  │  OIDC (brokered;     │
│  RBAC grants, cookie │  │  registry is a       │
│  sessions, audit log │  │  single confidential │
│                      │  │  client)             │
└──────────────────────┘  └──────────────────────┘
                          ┌──────────────────────┐
                          │   OTel Collector     │
                          │  → Jaeger / Prom /   │
                          │    Loki              │
                          └──────────────────────┘
```

The registry is the single source of truth: it owns the API, the SPA is only a
client of it, and PostgreSQL holds all registry state. The IdP is used only to
authenticate users (see [Auth Model](#2-auth-model)).

Router, middleware chain, and handlers: see `server/internal/http/`.
API surface: see `server/api/openapi.yaml` (served live at `/openapi.yaml`).

### 1.2 Request Model

The middleware chain (in order) is: OTel trace → request-id → CORS →
rate-limit → session auth → per-route publisher-role / reviewer guard. Read
endpoints are public by default; write endpoints require an authenticated
session and an effective role on the owning publisher (or Server Admin).
Effective roles are resolved live from `role_grants` on every request, not
trusted from the session. Wiring and per-route guards live in
`server/internal/http/` and `server/internal/auth/`.

### 1.3 Deployment Topology

**Development (docker-compose)** — Postgres, Keycloak, the Go server (hot
reload), the Vite dev server (HMR, proxying `/api/*` to the server), and the
OTel collector + Jaeger. See `deploy/docker-compose.yml` +
`deploy/docker-compose.dev.yml`.

**Production (docker-compose)** — Postgres (managed or container with a
volume), the server as a distroless image, the built SPA served by nginx (which
also proxies `/api/*` and `/config.json` to the server), a reverse proxy for TLS
termination, and an OTel collector exporting to external Prometheus / Grafana /
Tempo.

**Kubernetes (Helm chart)** — server and web Deployments (multiple replicas,
HPA on the server), a Service + Ingress with cert-manager TLS, PodDisruption
Budgets, an ExternalSecret for Postgres creds and the OIDC client secret, and a
ServiceMonitor for Prometheus scrape. See `deploy/helm/`.

---

## 2. Auth Model

Authentication and authorization are deliberately split, and the registry is the
**single token authority** — there is no multi-issuer validation and the SPA
never holds an IdP token.

**Two front doors, one session.** Users either log in with local email +
password or via brokered OIDC. OIDC is brokered **server-side**: the registry is
a single **confidential** OIDC client (Keycloak in dev). The browser hits
`/api/v1/auth/oidc/login`, the *server* runs the Authorization Code + PKCE flow,
exchanges the code with its `client_secret`, and maps the external identity onto
an internal `users` row. The IdP token never reaches the browser. Both front
doors end in the same registry-issued session behind a `Secure; HttpOnly`
cookie (BFF pattern). The opaque cookie token is never stored — only its
SHA-256 hash is the lookup key, so a DB leak yields no usable session.

*Why brokered + cookie sessions:* keeping the confidential client and all token
handling on the server means no client secret in the browser, no
`oidc-client-ts`, and one place that owns token lifetime. It also lets the
registry run with **no external IdP at all** (the bootstrap admin logs in
locally), which matters for self-hosted single-host installs.

**Claims carry group membership only.** At login the session **snapshots** the
OIDC claim group membership and the claim-based Server-Admin flag. There is no
claim-to-role side channel: roles are grants stored in the registry. This keeps
exactly two principal types (user, group) and avoids the IdP dictating
authorization.

**Authorization is publisher-scoped RBAC.** Roles (Viewer / Editor / Reviewer /
Admin) are granted to users or groups, scoped to a publisher (or globally when
the grant has no publisher). Per the resolved decisions: **Reviewer is the sole
approver** — a publisher Admin can do everything *except* approve a version
(separation of duties by default). Server Admin is the break-glass exception and
comes from the `realm_access.roles` claim containing `admin` **or** a local
`users.is_server_admin` flag (so the bootstrap admin works with no IdP).

Implementation: `server/internal/auth/` (broker, session, RBAC guards) and
`server/internal/domain/rbac.go`.

---

## 3. Observability Design

Principles (the mechanics — span names, metric names, attributes — live in code
and should be read there):

- A single OTel SDK setup in `server/internal/observability/` provides the
  `TracerProvider`, `MeterProvider`, and `LoggerProvider`, wired into
  `context.Context` at startup and never created ad-hoc.
- **Every HTTP handler is traced**; every DB call produces a **child span** of
  the request span. Exported functions that touch the network or DB take a
  `context.Context` and propagate it.
- **Structured logs always carry `trace_id` and `span_id`** for log-to-trace
  correlation in the collector pipeline. Logs are JSON via `slog` with an OTel
  bridge; diagnostic verbosity is controlled only by the log level. Never log
  the session cookie value, the `Authorization` header, or a raw IdP token.
- Propagation is W3C TraceContext.

Span names, metric definitions, and label sets: see
`server/internal/observability/` and the handlers. Export configuration is via
the standard `OTEL_*` env vars (documented in `deploy/.env.example`).

---

## 4. Data Lifecycle

### 4.1 Entity Relationships

```
publishers ──< mcp_servers ──< mcp_server_versions
           │
           └─< agents      ──< agent_versions

publishers ──< role_grants >── users / groups   (authorization)
groups     ──< group_members >── users
users      ──< sessions                          (registry cookie sessions)

audit_log (polymorphic: resource_type + resource_id, includes synthetic
           bootstrap-loader events)
reports   (polymorphic: target_type + target_id; admin triages)
```

Every MCP server / agent belongs to exactly one publisher; resources are
publisher-scoped (no workspace layer). Slug uniqueness is publisher-scoped — two
publishers can each own a `files` server, but a single publisher cannot own two.
Authorization is publisher-scoped RBAC: `role_grants` ties a role to a user or
group on a publisher (or globally when the grant has no publisher).

Schema (columns, types, indexes): see `server/migrations/`.

### 4.2 Version Lifecycle State Machine

Two **orthogonal** axes. The publish-status axis drives what public readers see;
the review-state axis decides *who* may make the next publish-axis transition.
This separation is the non-obvious bit — keeping them independent is what lets a
version be edited and re-reviewed without ever silently mutating something the
public has already cached.

**Publish axis**

```
   draft ── :publish ──▶ published ── :deprecate ──▶ deprecated
 (created)  (admin or    (immutable;                 (still readable;
            reviewer-     no metadata                  hidden from
            approved)     edits after                  default listing)
                          published_at)
```

Published versions are immutable — no edits to a version row once
`published_at` is set, because consumers cache agent cards and server metadata.

**Review axis** (change-approval workflow)

```
        edit / create
              │
              ▼
   ┌───────▶ none ◀────────┐
   │          │            │ :approve (Reviewer only;
   │ :reject  │ :submit    │  also runs the publish-axis transition)
   │          ▼            │
   │     pending_review ───┘
   │          │ :withdraw (author)
   ▼          │
 rejected ────┘
   │  :resubmit
   │  edit (revision++)
   ▼
  none   ← rejected versions are still drafts; another submit
           moves them back to pending_review.
```

Publisher Editors drive `none → pending_review` and the `pending_review → none`
withdraw. Only Reviewers approve (`pending_review → none`, which also fires the
publish-axis transition) or reject. **Every transition increments the row's
`revision`**, so concurrent edits surface a discriminated `409`
(`review-revision-mismatch`) instead of clobbering each other, and only one
version per entry may be pending at a time. Entry deletion uses the same review
queue, scoped to the entry rather than a single version.

Agents mirror MCP servers exactly, including this change-approval column set.

State-machine implementation and the discriminated `409` error types: see
`server/internal/domain/` and `server/internal/http/handlers/review.go`. Error
responses follow RFC 7807; the full catalogue is in `server/api/openapi.yaml`.

### 4.3 Pagination

All list endpoints use cursor-based pagination (an opaque base64 cursor encoding
`(created_at, id)` for stable ordering) rather than page numbers, so that
inserts don't shift pages under a paging client. Shapes are in
`server/api/openapi.yaml`.

---

## 5. UI/UX Design

**Framework**: Vite + React 19 + React Router v7 + shadcn/ui + Tailwind CSS v4,
shipped as a static SPA; nginx serves the bundle and proxies API paths to the Go
backend. Design tokens (color, typography, spacing, radii), the component
inventory, and responsive breakpoints are all derivable from `web/src/` and the
Tailwind config — read them there rather than from a table here.

The non-obvious UX decisions worth recording:

- **No bundled webfont.** Tailwind's default system stacks (`font-sans`,
  `font-mono`) keep first paint fast and the bundle small; the visual cost is
  acceptable for a developer-facing registry.
- **Dark mode** is toggled by a local `ThemeProvider` adding `class="dark"` on
  `<html>` (no `next-themes`); shadcn's CSS-variable system handles the swap.
- **Public UI is read-only**, Admin UI is CRUD — the API-first split is mirrored
  in the front end. `/admin/*` is guarded by a `<RequireAuth>` wrapper;
  unauthenticated visits redirect to login. `login()` either redirects to the
  server's brokered `/api/v1/auth/oidc/login` or POSTs the local
  `/api/v1/auth/login`; identity + grants come from `GET /api/v1/me`.
- **Listing pages paginate with a "Load more" button** (append), not page
  numbers — consistent with the cursor pagination model.
- **The Admin sidebar's Review-queue badge is live**: a TanStack Query hook polls
  `/api/v1/review-queue` on a 30s interval (shows "99+" past 99) and its cache is
  invalidated on every change-approval mutation toast, so the count stays
  current without a manual refresh.
- **The Review queue is Reviewer-only** (gated by `RequireReviewer`;
  non-reviewers get a 403 page), reflecting the separation-of-duties rule that
  Admins cannot approve.
- **Admin forms use plain `FormData` parsing**, not react-hook-form / zod — the
  forms are simple enough that the extra dependencies don't earn their keep.
  Validation errors render inline, scoped to the action that produced them, each
  region carrying `role="alert"`.
- **Destructive actions** use quiet styling (so they don't drown out a row's
  primary actions) plus a `window.confirm` gate as the real safety net.
- **Toasts** (`sonner`, top-right) fire on every change-approval and CRUD
  mutation; the same mutation invalidates the review-queue badge cache.

Accessibility is a hard requirement, not a tier: WCAG AA contrast on all
text/background pairs, visible focus rings on every interactive element,
semantic landmark elements, ARIA labels on icon-only buttons, `aria-current` on
active nav links, and keyboard-navigable command palette and menus.

Components and where they live: see `web/src/`.
