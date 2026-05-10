# ADR 0004 — Migrate the web app from Next.js to Vite + React SPA

- **Status:** Accepted (shipped during Phase 6; tracked in PLAN.md § Phase 6)
- **Date:** 2026-04-15 (backfilled 2026-05-10 — see "Why this ADR is late")
- **Deciders:** @Haibread
- **Supersedes:** —
- **Followed by:** —

## TL;DR

Rip out Next.js. Replace it with a plain Vite + React Router v7 + TanStack
Query SPA served as static files from nginx. NextAuth is replaced by
`oidc-client-ts` PKCE (no client secret). The UI components, Tailwind
build, and the OpenAPI-generated client all carry over unchanged.

## Why this ADR is late

`PLAN.md` § 7 (Definition of done) says "ADR if a cross-cutting decision
was made". Phase 6 was a cross-cutting decision — it changed the
framework, the auth model, the production runtime, and the docker image
shape — but no ADR was written at the time. The 2026-05-10 audit
flagged the gap. This ADR backfills the rationale; the implementation is
already in main.

## Context

The web app started life as a Next.js App Router project with NextAuth
for OIDC, Server Components for data fetching, and Server Actions for
mutations. By the time Phase 6 came around the registry had matured to
a clear shape:

- **Public browse**: read-only catalogue of MCP servers and agents.
- **Admin SPA**: CRUD on publishers, workspaces, MCP servers, agents.
- **Public API** does the heavy lifting; the web app is a thin client.

The Next.js stack was carrying weight we were not using:

- **No SEO requirement.** The catalogue is not Google-targeted.
  Public detail pages have JSON API mirrors at `/api/v1/...` that
  bots can crawl directly.
- **No static generation need.** Every list and detail page hits the
  API per request; nothing is build-time materialisable.
- **No SSR benefit.** Hydration bugs, double-fetch traps, and the
  Server Components / client boundary became net-negative for a UI
  that only renders after auth.
- **NextAuth.js v5 beta added a client secret to the SPA.** That
  contradicts the OAuth 2.1 PKCE public-client model the MCP
  authorization spec requires. The dev workflow worked around it
  with a configured Keycloak confidential client; production would
  have required rethinking it anyway.
- **Server Actions blurred the API-first boundary.** CLAUDE.md
  non-negotiable: every UI action maps 1:1 to an API call. Server
  Actions made it too easy to backdoor a mutation through a
  non-OpenAPI-documented path.

The team's TypeScript / React fluency was the same either way. The
deployment stack was already containerised — switching from
`node:22-alpine + next start` to `nginx:alpine + static files` was a
strict simplification.

## Decision

**Replace Next.js with a Vite + React Router v7 + TanStack Query v5
SPA.** Serve as static files from nginx in production. Use
`oidc-client-ts` PKCE (no client secret) for auth. Keep all UI
components, the Tailwind build, and the OpenAPI-generated client.

### What stays the same

- All UI components (Radix UI, shadcn/ui, Tailwind CSS, Lucide).
- `openapi-fetch` / `openapi-typescript` generated client.
- All page structure and visual design.

### What changes

| Area                | Before (Next.js)                    | After (Vite + React)                |
|---------------------|-------------------------------------|-------------------------------------|
| Routing             | App Router file-based               | React Router v7 `createBrowserRouter` |
| Data fetching       | Server Components + `getPublicClient` | `useQuery` (TanStack Query)        |
| Auth                | NextAuth.js + middleware            | `oidc-client-ts` + React context    |
| Admin guard         | `proxy.ts` middleware               | `<RequireAuth>` wrapper component   |
| Mutations           | Server Actions                      | `useMutation` + `fetch`             |
| Theme switching     | `next-themes`                       | Local `ThemeProvider`               |
| Dev proxy           | `next.config.ts` rewrites           | Vite `server.proxy` config          |
| Production serving  | Node.js (`next start`)              | nginx static file server            |
| Docker image        | `node:22-alpine` + standalone       | `nginx:alpine` (static files only)  |
| Page metadata       | `export const metadata`             | `<title>` via React Router future flag |

### Auth model

`oidc-client-ts` runs as a **public** OIDC client with PKCE — no client
secret, no `AUTH_KEYCLOAK_SECRET`. Tokens are stored in
`sessionStorage` by default (per-tab, cleared on close); `localStorage`
is an opt-in for E2E because Playwright's `storageState()` only
captures `localStorage`. The `auth_storage` knob in `/config.json`
controls which store the SPA uses.

### Production serving

Two-stage Docker build: `node:22-alpine` builds the static bundle,
`nginx:alpine` serves it. Nginx config does:

- `try_files $uri /index.html` for client-side routing.
- Proxy `/api/v1/*`, `/v0/*`, `/.well-known/*` to the server.

The dev experience uses Vite's `server.proxy` for the same paths so
no CORS headers are needed locally.

## Consequences

### Positive

- **Smaller production image.** `nginx:alpine` ≪ `node:22-alpine + next standalone`.
- **No SSR class of bugs.** Hydration mismatches, double fetches,
  Server Action ↔ Client Component boundaries — all gone.
- **API-first boundary is easier to enforce.** Every mutation is a
  `useMutation` against an OpenAPI-documented endpoint. Server
  Actions can no longer sneak in a side door.
- **Auth is honest about its threat model.** PKCE public client with
  no secret matches the MCP authorization spec; `sessionStorage`
  tokens shrink the XSS-exfiltration window vs `localStorage`.
- **Faster local dev.** Vite dev server beats Next.js dev server on
  cold start and HMR for our app size.

### Negative

- **No SSR escape hatch.** If a future requirement forces SEO on
  detail pages, we would either re-add a tiny static generator or
  rely on the JSON API for crawler use.
- **No Server Actions.** All mutations now go through the API; the
  one-line `await action(formData)` pattern was convenient.
- **First paint depends on JS.** Public visitors download the SPA
  bundle before they see content. Bundle size is small (largest
  pre-gzip chunk ~256 kB) but it's not zero.
- **NextAuth's session cookie pattern is gone.** The server only
  validates JWTs now; if we ever add server-side sessions we'd
  re-introduce that machinery.

### Neutral

- **All UI work, all design decisions, all component tests carry
  over unchanged** because we kept the React + Tailwind + shadcn/ui
  layers and only swapped routing / data fetching / auth / serving.
- **Coverage floors and CI gates moved 1:1.** The 80% statement
  floor on admin pages set in v0.2.2 still applies.

## Alternatives considered

1. **Stay on Next.js, fix the pain in place.** Replace NextAuth with
   `oidc-client-ts` inside the existing project; demote Server
   Components / Server Actions; configure the Next.js standalone
   output more carefully. Rejected because the cost is similar to a
   migration but we keep paying SSR complexity and a node-server
   image forever.
2. **Remix.** Same SSR model, same hydration class of bugs. Rejected
   for the same reasons we rejected Next.js — we don't want any
   server-rendering layer.
3. **Astro.** Tempting for the public catalogue (zero JS by default).
   Rejected because the admin half of the app is a thoroughly dynamic
   SPA, and running two web frameworks for one site doubles the build
   surface.
4. **Plain CRA / `react-scripts`.** Rejected — unmaintained, slow dev
   server, no first-class TypeScript story for our needs.
5. **SvelteKit / SolidStart.** Rejected — the team's React investment
   is the cheap part of the stack to keep.

## Out of scope

- **PWA / offline support.** Not asked for; not landed.
- **Server-side rendering for SEO.** Re-evaluate only when we see a
  real SEO ask we can act on.
- **i18n.** Not yet a requirement; the framework swap doesn't make
  it harder.

## Implementation sketch (historical)

The migration shipped in seven steps, all already complete on `main`:

1. **Scaffold.** Vite + React + TypeScript + Tailwind v4 in `web/`.
   Migrate `src/components/ui/` and `src/lib/` (no Next.js imports).
2. **Auth.** `AuthProvider` using `oidc-client-ts` with PKCE,
   `<RequireAuth>` wrapper, silent renew via `automaticSilentRenew`,
   `AuthCallback` route at `/auth/callback`. Drop
   `AUTH_KEYCLOAK_SECRET`.
3. **API client.** Single `useApiClient()` hook (public: no headers;
   authed: Bearer token). All admin pages use `useQuery` /
   `useMutation`.
4. **Routing.** React Router v7 `createBrowserRouter` in `src/main.tsx`.
   Public + admin + auth-callback routes defined.
5. **Page conversion.** All pages converted to client components.
   `next/link` → `react-router-dom` `<Link>`; `usePathname` /
   `useRouter` / `useSearchParams` → React Router equivalents;
   `notFound()` / `redirect()` → React Router primitives.
6. **Production build.** `web/nginx.conf` with `try_files $uri
   /index.html` + server-proxy blocks. Dockerfile is two-stage:
   `node:22-alpine` build → `nginx:alpine` serve.
7. **Cleanup.** Remove `src/app/`, drop `next` / `next-auth` /
   `next-themes` from `package.json`, update `CLAUDE.md` and
   `PLAN.md` to reflect the new stack.

## References

- [PLAN.md § Phase 6](../../PLAN.md) — migration roadmap and step list.
- [CLAUDE.md](../../CLAUDE.md) — declared stack ("Vite + React Router
  v7 + TanStack Query v5 + TypeScript + shadcn/ui + Tailwind. A pure
  SPA served as static files from nginx").
- [oidc-client-ts](https://github.com/authts/oidc-client-ts) — PKCE
  public-client OIDC library.
- [MCP Authorization spec](https://modelcontextprotocol.io/) — OAuth
  2.1 / OIDC with PKCE; the SPA-side public-client requirement.
- [PR #42 — react-hooks v7 cleanup](https://github.com/Haibread/ai-registry/pull/42),
  [PR #54 — post-audit doc/dead-code cleanup](https://github.com/Haibread/ai-registry/pull/54) — follow-on work that
  removed the last residual Next.js shaped artefacts.
