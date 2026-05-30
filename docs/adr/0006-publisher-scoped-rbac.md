# ADR 0006 — Publisher-scoped RBAC, removing workspaces

- **Status:** Accepted (2026-05-29); implemented 2026-05-30 (PRs #79–#85)
- **Date:** 2026-05-29
- **Deciders:** @Haibread
- **Supersedes:** [ADR 0001 — Workspaces under publishers](0001-workspaces-under-publishers.md),
  [ADR 0002 — Workspace OIDC group binding](0002-workspace-group-binding.md)
- **Amends:** [ADR 0003 — Change-approval workflow](0003-change-approval-workflow.md)
  (the reviewer authorization gate; the workflow itself is unchanged)

## TL;DR

Adopt a role-based authorization model and **remove the workspace
layer entirely**.

- Resources hang **directly off publishers** again. The `workspaces`
  table and `workspace_id` columns are dropped; `publisher_id` returns
  to `mcp_servers` / `agents`.
- **Publisher = the tenant ("org")**. Authorization is a set of
  **role grants** scoped to a publisher.
- Roles: **Viewer / Editor / Reviewer / Admin** per publisher, plus a
  global **Server Admin**.
- A grant's principal is a **user** or a **group** (team) — those are the
  only two principal types. Users, groups, and grants all live in the
  registry's own tables.
- **Two kinds of accounts, handled together:** *federated* users (log in
  via OIDC, provisioned just-in-time) and *local* users (log in with a
  registry password). One person can be both. Likewise a group can be
  local-only or fed from a token claim.
- Custom claims carry **group membership only**: a claim value matching a
  local group's `slug` makes the caller a member. The chain is **claim →
  group → role on a publisher**; the group↔role binding is an API-managed
  grant on the registry side. No claim-to-role side channel.
- **MCP compatibility improves**: dropping workspaces returns `/v0/` to
  the bare publisher-namespaced spec shape. Local (password) tokens are
  walled off from the MCP surface, which stays OAuth-only.

## Context

Three things converged:

1. **Workspaces no longer earn their keep.** ADR 0001 inserted a
   workspace layer between publishers and resources; ADR 0002 used it as
   the unit of write-delegation (one Keycloak group per workspace). In
   practice the layer adds a mandatory concept, two-level slug
   uniqueness, hierarchical URLs, and `LookupGroupNameBy*` plumbing — for
   a granularity (intra-publisher delegation) that the registry does not
   actually need. The MCP registry spec has no workspace concept; it
   namespaces by publisher.

2. **Authorization should be the registry's, managed in one place.**
   ADR 0002's own "Negative" section flagged the core problem: group
   membership lives in Keycloak, the binding lives in the registry, two
   consoles, drift. We want to define roles and grant them through the
   registry's own API/UI.

3. **The target is a role-based model** (per the design discussion):
   local *and* federated accounts + teams, and custom-claim → (org, role)
   mapping. Publisher is the natural tenant ("org").

**Authentication** supports two front doors, handled together: OIDC
(federated) and a local email + password login the registry issues a
token for. The MCP authorization spec (CLAUDE.md core principle #4)
requires OAuth 2.1 / OIDC on the MCP write surface, so **local tokens are
accepted only on the human/admin API and are rejected on the MCP
surface**. We also move **authorization** fully in-house.

## Decision

### 1. Remove the workspace layer

Resources return to being publisher-scoped. This reverses ADR 0001's
finalising migration (`000011`):

- Re-add `publisher_id` to `mcp_servers` and `agents`, backfilled from
  `workspaces.publisher_id` via the existing `workspace_id` join.
- Restore `UNIQUE (publisher_id, slug)`; drop `UNIQUE (workspace_id,
  slug)`.
- Drop `workspace_id` columns (and their indexes/FKs) and the
  `workspaces` table.
- Canonical resource URLs drop the workspace segment:
  `/v0/publishers/{p}/servers/{s}`. This URL form **already exists**
  today as the legacy 301-redirect *source* (ADR 0002 redirected it
  *into* `/workspaces/default/...`); we promote it back to canonical and
  emit 301s from the `/workspaces/...` form for a deprecation window.

The slug-uniqueness change is normally benign — every publisher had a
single `default` workspace in practice, so `(workspace_id, slug)` collapses
to `(publisher_id, slug)` cleanly. The migration must not *assume* it: before
re-adding the unique key it scans for duplicate `(publisher_id, slug)` across
a publisher's workspaces and `RAISE EXCEPTION`s with the offending rows if
any exist (mirroring `000011`'s backfill gate), so a multi-workspace
deployment fails loudly instead of losing a row.

### 2. Identity: local and federated principals

```sql
CREATE TABLE users (
    id              text PRIMARY KEY,        -- ULID, internal — THE principal key
    email           text NOT NULL UNIQUE,    -- human key
    display_name    text,
    subject         text UNIQUE,             -- OIDC `sub`; NULL for local-only users
    password_hash   text,                    -- argon2id; NULL for OIDC-only users
    is_server_admin boolean NOT NULL DEFAULT false,
    disabled        boolean NOT NULL DEFAULT false,
    last_seen_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
```

A **principal is a `users` row, keyed internally by `id`** — *not* by the
OIDC subject. A row may carry `subject` (can log in via OIDC),
`password_hash` (can log in locally), both, or neither.

**Local vs. federated — both first-class:**

- **Local user.** Created by an admin with an email and a password;
  `subject` stays `NULL`. Authenticates at the local login endpoint
  (§3). Its groups come only from local `group_members`.
- **Federated user.** Logs in via OIDC. Resolved by `subject`; on first
  login, if the token's email is **verified** it binds `subject` to a
  matching pre-invited row (**bind-once** — never rebind on a later email
  change; this relinking path is a known account-takeover CVE class),
  otherwise a row is lazy-created (JIT). Its groups come from
  `group_members` **and** claim-matched groups.
- **Linked.** A federated login whose verified email matches an existing
  local user binds onto that one row — afterwards either front door works
  for the same principal.
- **Invited.** A row with neither `subject` nor `password_hash` (e.g.
  created by adding an email to a group): it can hold grants and
  memberships, but cannot log in until a credential is set.

`disabled = true` is a local kill-switch enforced on every authenticated
request, whichever front door was used. **Bootstrap admin:** one local
account is seeded from config (`is_server_admin = true` + an initial
password) — **created only if absent; the seed never overwrites an existing
account's password**, so a rotated password survives reboots. This makes the
registry usable before any IdP is wired and means you can never lock
yourself out.

### 3. Authentication (two front doors, one principal)

Both login methods resolve to a single `users.id`, then share the same
authorization path.

- **OIDC (federated).** Unchanged validation: bearer JWT verified against
  the IdP's JWKS, with issuer + audience checked.
- **Local (password).** `POST /api/v1/auth/login` takes email + password,
  verifies the argon2id hash, and returns a short-lived token **signed by
  the registry's own key**. The registry thus becomes a second token
  issuer and exposes its own JWKS for self-verification.

`Authenticate` becomes **multi-issuer**: it inspects the token's `iss` and
routes verification to the IdP's JWKS or the registry's local key, then
resolves the principal (`users.id`) and loads effective groups + role.

**MCP wall (non-negotiable).** Locally-issued tokens carry a distinct
issuer/audience marker and are **accepted only on the human/admin API**.
The MCP + agent protocol routes validate against the OIDC issuer only and
reject local tokens — preserving CLAUDE.md #4. A leakage test asserts a
local token is refused on `/v0/...` write paths (mirroring
[v0mcp_review_leakage_test.go](../../server/internal/http/handlers/v0mcp_review_leakage_test.go)).

Local-auth configuration (env + YAML + default per CLAUDE.md):

| Env var                         | YAML key                 | Default |
|---------------------------------|--------------------------|---------|
| `AUTH_LOCAL_LOGIN_ENABLED`      | `auth.local_login`       | `true`  |
| `AUTH_LOCAL_SIGNING_KEY`        | `auth.local_signing_key` | —       |
| `AUTH_LOCAL_TOKEN_TTL`          | `auth.local_token_ttl`   | `1h`    |
| `AUTH_BOOTSTRAP_ADMIN_EMAIL`    | `auth.bootstrap_admin_email` | —   |
| `AUTH_BOOTSTRAP_ADMIN_PASSWORD` | (env / secret only)      | —       |

The signing key is required when local login is enabled (generated in dev,
supplied via secret in prod); the bootstrap password is consumed at first
boot and should be rotated. Both are credentials — env/secret only, never
in a committed config file (secrets conventions).

### 4. Authorization: roles, principals, grants

```sql
CREATE TABLE groups (              -- "teams"
    id          text PRIMARY KEY,  -- ULID
    slug        text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE group_members (
    group_id   text NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    text NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE role_grants (
    id             text PRIMARY KEY,                 -- ULID
    principal_type text NOT NULL CHECK (principal_type IN ('user','group')),
    principal_id   text NOT NULL,                    -- users.id or groups.id
    publisher_id   text REFERENCES publishers(id) ON DELETE CASCADE,  -- NULL = all publishers
    role           text NOT NULL CHECK (role IN ('viewer','editor','reviewer','admin')),
    source         text NOT NULL DEFAULT 'api' CHECK (source IN ('api','config')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    -- NULLS NOT DISTINCT (PG15+) makes a global grant (publisher_id NULL)
    -- collide with itself, so duplicates are rejected without a sentinel.
    UNIQUE NULLS NOT DISTINCT (principal_type, principal_id, publisher_id, role)
);
```

A grant says: *principal P holds role R on publisher Q* — or on **all
publishers** when `publisher_id` is `NULL`. The principal is a **user** or
a **group** (applies to every member). `source` records whether the row was
created via the API or seeded from config (the reviewer-group seed, §5).
`principal_id` is polymorphic, so it carries no FK — the store deletes a
principal's grants when its user or group is removed.

#### Roles

| Role        | Scope     | Capabilities it grants                                                     |
|-------------|-----------|---------------------------------------------------------------------------|
| Viewer      | publisher | Read **private** entries of the publisher (public reads need no role).    |
| Editor      | publisher | Viewer + create / edit / submit resources and versions.                   |
| Reviewer    | publisher | Viewer + approve / reject submitted versions and pending deletions.       |
| Admin       | publisher | Editor + Reviewer + edit publisher metadata + assign role grants.         |
| Server Admin| global    | Everything, all publishers; create/delete publishers; grant Server Admin. |

**Editor and Reviewer are independent** — neither implies the other. The
roles form a lattice (Viewer ⊂ {Editor, Reviewer} ⊂ Admin), so by default
editors and reviewers are *different people* (separation of duties). A user
gains both only by being granted both — a deliberate act — and may then
approve their own change; we accept that as an explicit trust decision
rather than forcing a `submitted_by != reviewed_by` guard (that stays an
optional policy, F3).

**Server Admin** comes from either source: the OIDC claim
(`realm_access.roles[]` contains `admin`, Decision A) **or** a local
`users.is_server_admin = true` flag. The flag is what lets the seeded
local bootstrap admin operate with no IdP; other Server Admins can toggle
it. Claim-based admin remains the federated escape hatch.

#### Effective-role resolution

Once the principal is resolved to a `users.id` (via either front door), its
effective permissions on publisher `Q` are the **union of capabilities**
across every grant that applies (grants match when `publisher_id` is `Q`
**or** `NULL` = all-publishers):

1. grants to the caller's user (`users.id`);
2. grants to any of the caller's **effective groups**. Effective groups =
   local `group_members` ∪ (for OIDC tokens) claim-group names matched
   verbatim to a local `groups.slug`; local tokens contribute only
   `group_members`. This also means **existing Keycloak-group users keep
   access with no membership migration** — the claim still names the
   group; the group now carries the grant;
3. Server Admin — claim `admin` **or** `users.is_server_admin` → all
   capabilities.

Because Editor and Reviewer are independent, a caller can hold one, the
other, or both; holding **both** is the only way to author *and* approve
the same change.

### 5. Claim → authorization mapping

The custom-claim plumbing (`AUTH_GROUPS_CLAIM`, top-level JSON string
array, default `groups`) is retained, with **one job: team membership**. A
claim value that matches a local `groups.slug` makes the caller an
effective member of that group (alongside any local `group_members`).

What a group *can do* is decided entirely **on the registry side** by its
role grants. So the mapping is **claim → group** (membership), then
**group → role on a publisher** (an API-managed grant). There is no
claim-to-role side channel — the only principal types are **user** and
**group**, and every permission is a grant, so authorization stays fully
API-configurable (core principle #1). To let "anyone with claim value X"
hold a role, create a group whose slug is `X` and grant it that role.

| Env var             | YAML key            | Default  |
|---------------------|---------------------|----------|
| `AUTH_GROUPS_CLAIM` | `auth.groups_claim` | `groups` |

### 6. Middleware

- `Authenticate` ([server/internal/auth/middleware.go](../../server/internal/auth/middleware.go))
  becomes **multi-issuer** (IdP JWKS or the registry's local key, by
  `iss`) and gains a narrow store dependency: resolve the principal
  (`users.id`), enforce `disabled`, and stash the caller's effective
  groups in context. A claim-`admin` token needs no DB, so a
  membership-lookup failure **fails closed for members but never locks
  out a federated Server Admin**.
- New `RequirePublisherRole(role, resolvePublisher)` replaces
  `RequireWorkspaceWrite`. It resolves the request's publisher (from the
  `{ns}` path param or the referenced resource) and checks the caller holds
  the required role there — **Editor** for writes, **Reviewer** for
  approvals — with **Admin** (and **Server Admin**) satisfying either.
  Roles are a lattice, not a threshold, so this is a capability check.
- **Amends ADR 0003:** approve / reject / approve-deletion switch from
  `RequireReviewer(globalGroup)` to `RequirePublisherRole(Reviewer, …)`,
  and the `GET /review-queue` listing is filtered to publishers where the
  caller is Reviewer (Server Admin sees all). The change-approval
  *workflow* (states, `revision`) is otherwise untouched.
  `AUTH_REVIEWER_GROUP` is retained for back-compat: on boot it ensures a
  group of that name exists with a global (`publisher_id = NULL`) Reviewer
  grant. Deprecated in favour of managing that group + grant via the API.
- `RequireAdmin` (Server Admin) keeps: publisher create/delete, granting
  Server Admin, and global operations.

### 7. API surface (all in OpenAPI — non-negotiable)

- Auth: `POST /api/v1/auth/login` (email + password → registry token),
  `POST /api/v1/auth/logout`, `POST /api/v1/users/{id}:set-password`
  (self or Server Admin). OIDC login needs no endpoint here.
- Groups (Server Admin): `GET/POST /api/v1/groups`, `GET/PATCH/DELETE
  /api/v1/groups/{slug}`, `PUT/DELETE /api/v1/groups/{slug}/members/{email}`
  (member-add auto-creates the `users` row if the email is unknown).
- Users (Server Admin): `GET/POST /api/v1/users` (create a local / invited
  user), `GET /api/v1/users/{id}`, `PATCH /api/v1/users/{id}` (display
  name, `disabled`, `is_server_admin`).
- Grants: `GET/POST/DELETE /api/v1/publishers/{slug}/grants` — per-publisher
  grants for `user` / `group` principals (publisher Admin or Server Admin).
  `GET/POST/DELETE /api/v1/grants` — global / all-publisher grants
  (`publisher_id = NULL`; Server Admin only). Both expose `source`;
  config-seeded rows (the reviewer group's grant) are re-applied on boot,
  so deleting one via the API only sticks if the seed is also removed.
- Resource paths drop `/workspaces/{w}` everywhere; `workspace` request
  fields are removed. The `Workspace*` schemas are deleted.

### Authorization matrix

| Action                                         | Required                          |
|------------------------------------------------|-----------------------------------|
| Read public entries                            | Public                            |
| Read private entries of a publisher            | Viewer (or higher) on publisher   |
| Create / edit / submit a server / agent / ver. | Editor on publisher               |
| Approve / reject a version; approve deletion    | Reviewer on publisher             |
| Edit publisher metadata; assign role grants    | Admin on publisher                |
| Manage groups, users, and global grants        | Server Admin (global)             |
| Create / delete publishers; grant Server Admin | Server Admin (global)             |

### Audit

The existing `audit_log` records the actor (`actor_subject`, `actor_email`)
on every HTTP call. Extend it so the new security-sensitive mutations —
role grant/revoke, `disabled` toggles, `set-password`, and `is_server_admin`
changes — also capture the **target** (which user / group / grant). "Who
granted whom what, and when" is the first question in an incident, so it
must be answerable from the log.

## Migration & cutover

Forward-only, mirroring the additive→finalise pattern ADR 0001 used.

1. **`000012` (additive):** create `users`, `groups`, `group_members`,
   `role_grants`. Re-add nullable `publisher_id` to resources, backfill
   from `workspaces.publisher_id`. Convert each `workspaces.group_name`
   binding into: a `groups` row (slug = the name) + a `role_grant(group,
   publisher, 'editor')`. On boot, seed the bootstrap admin and the
   `AUTH_REVIEWER_GROUP` group with a global (`publisher_id = NULL`)
   Reviewer grant (`source = 'config'`). Swap the write/review routes onto
   `RequirePublisherRole` in the same change — a clean cutover, no
   dual-running guards; the parity test (step 2) is the safety net.
2. **Verify** the converted grants reproduce prior access (parity tests:
   the same tokens that could write before can write after).
3. **`000013` (finalise):** flip `publisher_id` NOT NULL, restore
   `UNIQUE (publisher_id, slug)` (gated by the slug-collision check above),
   drop `workspace_id` + `workspaces`,
   delete the workspace code/UI, retire the old guards and the
   `/workspaces/...` redirects (or keep 301s for one release).

Because effective-group resolution matches claim group names to local
`groups.slug`, **no user-membership migration is required** — existing
Keycloak-group users keep working through the converted grants.

**Delivery.** Even as one ADR, ship it as two PRs: **(A)** add RBAC + local
auth and re-add `publisher_id` *additively* (it coexists with
`workspace_id`), swapping the guards onto `RequirePublisherRole`; **(B)**
remove workspaces (drop `workspace_id` + `workspaces`, retire the old
routes). Each PR stays reviewable; the parity test guards the boundary.

## Consequences

### Positive

- Simpler model: Publisher → resources, roles on publisher. One fewer
  mandatory concept; no two-level slug uniqueness.
- **More MCP-spec-faithful**: `/v0/` returns to bare publisher
  namespacing.
- Authorization lives in one place (registry API/UI); resolves ADR
  0002-F2 ("who can write to X?") for free — it's now a SQL query.
- A real role model (Viewer/Editor/Reviewer/Admin) instead of binary
  admin + group-gated write.
- Reversible-ish: grants default to none; with no grants and no claim
  mapping, only Server Admin can write — the pre-Phase-7 posture.

### Negative

- **Destructive revert of shipped Phase 7.1 work** (workspaces). Real
  surgery on a running stack; needs careful migration + full test pass.
- **Coarser delegation**: an Editor on a publisher can write *all* of
  that publisher's resources. Intra-publisher delegation (the workspace
  feature) is gone. Reintroducible later as **optional** folders (a finer
  per-folder permission layer) if a concrete need appears — not built now.
- **Reverses three documented stances**: (1) "no user table" (PLAN #7);
  (2) authentication fully delegated to the IdP with no stored credentials
  (now local passwords + a registry-issued token); and (3) softens "all
  writes go through admins" (CLAUDE #3) further than ADR 0002 already did.
  CLAUDE.md / PLAN.md need updating.
- `Authenticate` resolves the principal + effective permissions from the DB
  on **every** authenticated request — **no cache in v1**, so `disabled` and
  grant changes take effect immediately and we need no token-revocation
  list. A claim-`admin` token needs no DB. If profiling later shows this
  hurts, add a short per-principal cache then, accepting the revoke lag.
- **Local auth adds real security surface now, not later**: password
  hashing (argon2id), a signing key to manage and rotate, a second token
  issuer, multi-issuer validation, login lockout, and the MCP-wall
  leakage tests. This is the project's first stored credential — handle
  per the secrets conventions (env/secret only, never a config file or
  git).
- URL change is a public-API change (mitigated by 301s + pre-1.0 status).

### Neutral

- Change-approval workflow (ADR 0003) is unchanged except *who* may
  approve.
- Public reads unaffected.
- Server Admin is dual-sourced: the IdP claim for federated admins, the
  local `is_server_admin` flag for the bootstrap / local admins.

## Alternatives considered

1. **Keep workspaces as an optional "folder" sub-scope.** Rejected: a
   layer nobody is required to use is still a concept everyone must learn;
   cut it now, add optional folders later only if needed.
2. **Roles at workspace granularity (keep workspaces).** Rejected per the
   decision to remove workspaces; finer than the registry needs.
3. **A new top-level `org` entity above publisher.** Rejected: publisher
   already *is* the tenant; a new entity is pure cost.
4. **IdP-claim-only authorization (deeper ADR 0002 path: reconciler,
   SCIM).** Rejected: the goal is to stop depending on IdP group admin.
5. **Deferring local passwords to a later phase.** Considered — it would
   isolate the security-sensitive credential / token-issuer work — but
   **rejected**: local accounts must be usable from day one. Because the
   principal is keyed on `users.id` and is auth-method-agnostic, building
   both front doors together avoids a later rework of the auth layer, at
   the cost of taking on the credential surface now (see Consequences).

## Out of scope (FUTURE WORK)

- **F1. Optional folders** for intra-publisher delegation (a finer
  per-folder permission layer), only if a concrete need appears.
- **F2. Service-account / API-key principals** (parked v0.4.x): these map
  to a publisher + role, not a user/group; they bypass interactive login.
- **F3. Optional strict separation of duties** — a config toggle forcing
  `submitted_by != reviewed_by`, for deployments that want to forbid
  self-approval even when a user deliberately holds both Editor and
  Reviewer. Allowed by default (see §4); carries over from ADR 0003-F2.
- **F4. Account self-service & session UX**: password reset via email,
  email-based invitations / notifications, MFA on local login, and refresh
  tokens / sliding sessions (v1 ships a fixed-TTL local token, so local
  users re-login at expiry).
- **F5. Group nesting and SCIM sync.**

## Implementation sketch

1. **Migration `000012`** (additive): `users` (incl. `password_hash` +
   `is_server_admin`), `groups`, `group_members`, `role_grants`; re-add
   `publisher_id`, backfill, convert bindings → grants. Test mirrors
   [store/migrate_finalise_test.go](../../server/internal/store/migrate_finalise_test.go).
2. **Config + boot seed** — `AUTH_LOCAL_LOGIN_ENABLED`,
   `AUTH_LOCAL_SIGNING_KEY`, `AUTH_LOCAL_TOKEN_TTL`, and the bootstrap-admin
   creds, wired env/YAML/default in
   [server/internal/config/config.go](../../server/internal/config/config.go).
   On boot, idempotently seed the bootstrap admin and the
   `AUTH_REVIEWER_GROUP` group + its global Reviewer grant
   (`source = 'config'`); `deploy/.env.example` updated; `AUTH_REVIEWER_GROUP`
   marked deprecated.
3. **Store** — `store/users.go` (resolve by id/subject/email, password
   verify, set-password), `store/groups.go`, `store/grants.go`
   (user/group grants incl. global scope, effective-role resolution,
   reviewer-group seed reconciliation); delete `store/workspace.go`.
4. **Auth — federated** — multi-issuer `Authenticate`, JIT +
   email-bind-once + `disabled`, effective groups, `RequirePublisherRole`;
   retire `RequireWorkspaceWrite`; repoint the ADR 0003 reviewer gate.
5. **Auth — local** — argon2id hashing, the registry signing key + its
   JWKS, `POST /api/v1/auth/login`, login lockout, the local-token marker
   + the **MCP wall**, and Server Admin via `is_server_admin`.
6. **Handlers** — auth/login + set-password, groups / users CRUD, grants
   CRUD (per-publisher `/publishers/{slug}/grants` + global `/grants`);
   strip `/workspaces` from resource routes; delete `handlers/workspace.go`.
7. **OpenAPI** — add `Login`, `Group`, `User`, `RoleGrant` schemas +
   paths; delete `Workspace*`; remove the workspace path segment.
8. **Frontend** — a login screen (OIDC button **and** local email/password
   form); Groups, Users, and per-publisher Grants admin pages; delete
   `workspaces-section.tsx` and the workspace selectors.
9. **Tests** — store CRUD + effective-role matrix (user / group grants,
   per-publisher **and** global `NULL` scope); claim → group membership
   resolution; multi-issuer
   `Authenticate` (OIDC + local); JIT / email-bind-once / disabled;
   password verify + lockout; **MCP-wall leakage test** (local token
   refused on `/v0/...`); `RequirePublisherRole` matrix asserting the
   lattice — **Reviewer alone can't write, Editor alone can't approve**,
   Admin/Server-Admin satisfy both; migration parity (pre/post tokens write
   the same set).
10. **Docs** — update CLAUDE.md (decisions + reverse #7), PLAN.md
    (reverse decision #7, mark the workspace phases superseded, add the new
    phase), mark ADR 0001/0002 superseded and 0003 amended; expand
    `deploy/.env.example`.
11. **Migration `000013`** (finalise): `publisher_id` NOT NULL, restore
    `UNIQUE (publisher_id, slug)`, drop `workspace_id` + `workspaces`,
    retire old guards/redirects.

## References

- [ADR 0001](0001-workspaces-under-publishers.md) (superseded),
  [ADR 0002](0002-workspace-group-binding.md) (superseded),
  [ADR 0003](0003-change-approval-workflow.md) (reviewer gate amended).
- [CLAUDE.md](../../CLAUDE.md) — MCP OAuth requirement, configuration
  rule, and the no-user-table / IdP-only-auth decisions this reverses.
- [server/internal/auth/middleware.go](../../server/internal/auth/middleware.go),
  [claims.go](../../server/internal/auth/claims.go),
  [config.go](../../server/internal/auth/config.go).
