# ADR 0002 — Workspace OIDC group binding

- **Status:** Accepted (shipped via PR #29)
- **Date:** 2026-05-02
- **Deciders:** @Haibread
- **Builds on:** [ADR 0001 — Workspaces under publishers](0001-workspaces-under-publishers.md)
- **Followed by:** [ADR 0003 — Change-approval workflow](0003-change-approval-workflow.md)

## TL;DR

Each workspace can name **at most one** Keycloak group. Members can
author content for the workspace; admins can author for any workspace.

- `workspaces.group_name` (nullable; `NULL` = admin-only).
- JWT carries a `groups` claim. Configurable claim name
  (`AUTH_GROUPS_CLAIM`, default `groups`). Bare names, not Keycloak
  full paths.
- New `RequireWorkspaceWrite` middleware replaces `RequireAdmin` on
  resource-write endpoints. Admin override preserved.
- Manual Keycloak setup is the v1 answer. Reconciler ("operator") is
  future work F4.

## Phasing

This ADR depends on ADR 0001 having shipped: `workspaces.group_name`
has nowhere to live without the workspace table, and
`RequireWorkspaceWrite` resolves the workspace from a URL path that
doesn't exist pre-0001.

## Context

ADR 0001 introduced the workspace layer but kept all writes
admin-only. We want to delegate writes to non-admin users without:

- introducing a user table (rejected in
  [migrations/000001_init.up.sql:11](../../server/migrations/000001_init.up.sql)),
- adding a synchronous Keycloak Admin API call to the hot path,
- expanding the surface beyond what one Keycloak primitive (groups)
  can cover.

Keycloak groups are the natural fit: they represent "set of users",
they ship with a token mapper that emits the user's groups as a claim,
and they don't require minting a fresh realm role per workspace.

## Decision

### Schema

```sql
ALTER TABLE workspaces
    ADD COLUMN group_name text;

CREATE INDEX workspaces_group_name_idx
    ON workspaces (group_name)
    WHERE group_name IS NOT NULL;
```

Migration number is a placeholder; assigned at PR open time.

`group_name` is nullable — a workspace with `NULL` is admin-only,
preserving the post-0001 behavior. 1:1 binding for now; many-to-many
later is non-breaking (F3).

### JWT claim

Bare group names (e.g. `["claude-team", "registry-reviewers"]`), not
Keycloak full paths. Configured via a Keycloak group-membership mapper
with "Full group path" disabled. Token contents:

```json
{
  "sub": "…",
  "email": "…",
  "realm_access": { "roles": ["user"] },
  "groups": ["claude-team"]
}
```

Configurable per CLAUDE.md's env + YAML + default rule:

| Env var               | YAML key             | Default  |
|-----------------------|----------------------|----------|
| `AUTH_GROUPS_CLAIM`   | `auth.groups_claim`  | `groups` |

`KeycloakClaims` ([server/internal/auth/claims.go](../../server/internal/auth/claims.go))
gains:

```go
type KeycloakClaims struct {
    // … existing fields …
    Groups []string
}

func (c *KeycloakClaims) HasGroup(name string) bool { … }
```

`Groups` is populated from the configured claim name; missing or
non-array claims yield an empty slice (not an error — the claim is
optional from the server's perspective).

### Middleware

`RequireWorkspaceWrite(extractWorkspaceID)` replaces `RequireAdmin` on
resource-write endpoints. The extractor turns a request into a
`workspace_id`. The middleware:

1. Pulls claims from context.
2. Returns OK if `claims.IsAdmin()`.
3. Loads `workspace.group_name` for the request's workspace.
4. Returns 403 unless `claims.HasGroup(workspace.group_name)`.

For URL-path requests (`/v0/publishers/{p}/workspaces/{w}/servers/...`)
the extractor needs to resolve `(publisher_slug, workspace_slug) →
workspace_id`. For ULID-keyed requests (`/v0/servers/{ulid}/...`) it
resolves entry → workspace_id. Either way, **one DB lookup** per
write request before the handler runs. Acceptable; writes are rare.

### Redirect interaction

ADR 0001 keeps legacy `/v0/publishers/{p}/servers/{s}` paths working
via HTTP 301 redirects to
`/v0/publishers/{p}/workspaces/default/servers/{s}`. The redirect
emits a `Location` header and **does not** authorize the request — the
client follows the redirect and the auth check runs against the
*target* path. This avoids the redirect being a soft auth bypass
(e.g. an attacker hitting the legacy path expecting more permissive
handling).

### Existing `RequireAdmin` keeps these endpoints

- Publisher CRUD,
- Workspace CRUD (creating / renaming / deleting workspaces and
  binding groups).

Workspace creation is admin-only by design: the act of binding a
workspace to a group is the auth boundary itself, not a lever a
self-service user should hold.

### Authorization matrix

| Action                                  | Required principal                |
|-----------------------------------------|-----------------------------------|
| Create / edit / delete a publisher      | Admin                             |
| Create / edit / delete a workspace      | Admin                             |
| Set `workspaces.group_name`             | Admin                             |
| Create / edit a server or agent         | Workspace group member OR admin   |
| Create / edit a version                 | Workspace group member OR admin   |
| Anything not listed (read endpoints)    | Public per existing policy        |

### Manual Keycloak setup (v1)

For each workspace that should allow non-admin writes, an admin
performs four steps **in Keycloak's admin UI**:

1. Create a group (e.g. `claude-team`).
2. Add users to the group.
3. (Recommended) Add a description tying it to the workspace.
4. In the registry, set the workspace's `group_name` to match.

The Keycloak group-membership mapper that emits the `groups` claim
must also be configured once at realm setup. Without it no non-admin
can write — they'll see 403s. We surface a startup warning on a
heuristic: if any workspace has `group_name IS NOT NULL` and no
validated token has carried a non-empty `groups` claim within the
first ~100 requests after boot, log WARN once. The threshold is a
heuristic, not a contract; the warning is informational.

### Operator concerns (the "you'll need an operator" question)

A teammate raised that this design will eventually want a reconciler:
when a workspace is created, the corresponding Keycloak group should
be created automatically; on rename, renamed; on delete, cleaned up.
Without it, the registry and Keycloak hold two sources of truth and
inevitably drift.

We accept that pain for v1, because:

- At "tens of workspaces", manual Keycloak clicks are cheap. Drift is
  detectable (missing-group 403s are loud).
- Building a reconciler now means picking a flavor (in-process call to
  Keycloak Admin API on workspace CRUD; or a separate operator process
  watching state) and committing to a service-account in Keycloak with
  group-management privileges.
- The JWT-claim path (the read side: "is this user in the group") is
  zero-cost regardless of whether a reconciler exists.

The reconciler is captured as **F4** with two flavor sketches.
Pull-forward triggers: workspace count ≳ 50, or self-service
workspace creation requested.

### API surface

Existing endpoints, behavior changed:

- All MCP server / agent / version write endpoints under
  `/v0/publishers/{p}/workspaces/{w}/...` now use
  `RequireWorkspaceWrite` instead of `RequireAdmin`.

New field exposed:

- `WorkspaceAdmin` schema gains a nullable `groupName` field. Not
  exposed on the public `Workspace` schema.

OpenAPI MUST be updated accordingly per the API-first rule.

### Audit

Existing `audit_log` already records `actor_subject` and `actor_email`
from the JWT for every HTTP call — no schema change. The fact that
authorization went via group membership rather than admin role does
not need to be persisted; the per-action audit row plus the current
`workspace.group_name` value at action time is sufficient. Failed-auth
events (e.g. 403 because not in group) are also logged at the HTTP
layer; no separate audit pipeline.

## Consequences

### Positive

- Non-admin contributors can write to their workspace without
  realm-wide admin rights.
- Stateless model preserved.
- Reversible at every layer: a workspace with `group_name = NULL`
  behaves exactly as it did at the end of 0001.
- One Keycloak primitive (groups) covers the whole feature.
- Group renames in Keycloak are recoverable with a single SQL UPDATE
  on `workspaces.group_name`.

### Negative

- Operators must configure the Keycloak group-membership mapper, or
  the feature is silently dead. Documented in `deploy/.env.example`
  and surfaced via the boot-time warning.
- **Two systems hold related state** (registry: `workspaces.group_name`;
  Keycloak: groups + memberships). Manual setup invites drift. F4
  tracks the reconciler.
- The registry cannot answer "who can write to workspace X?" without
  a live Keycloak Admin API call. Out of scope (F2).
- 1:1 binding means a user belonging to multiple author groups still
  needs each workspace row updated individually if you want to remap
  (F3).
- `RequireWorkspaceWrite` adds one DB lookup per resource-write
  request. Acceptable.

### Neutral

- Admin override (`realm_access.roles[] contains "admin"`) preserved.
- API shape for existing endpoints is unchanged; only the *reason*
  for a 403 shifts.
- Public read endpoints are unaffected.

## Alternatives considered

1. **DB-backed user table with explicit workspace membership.**
   Rejected: contradicts the no-user-table decision and forces
   reconciliation with Keycloak on every login.
2. **Pure realm-role gating** (a role like
   `workspace:claude-team:write`). Rejected: couples workspace
   creation to Keycloak realm config — admins would have to mint a
   role per workspace. Group membership is the standard Keycloak
   primitive for "set of users".
3. **Live Keycloak Admin API lookup per request.** Rejected: adds a
   network hop, a service-account secret, and a cache layer to the
   hot path for no gain over reading the `groups` claim.
4. **Many-to-many workspace↔group from day one.** Rejected for now:
   no concrete need; non-breaking later (F3).
5. **Bind groups to publishers instead of workspaces.** Rejected:
   re-introducing the pre-0001 design defeats the reason 0001 added
   the workspace layer.
6. **Build a Keycloak reconciler in v1.** Rejected; tracked as F4
   with explicit pull-forward criteria.

## Out of scope (FUTURE WORK)

- **F1. Per-resource-type group binding** — distinguish "can write
  servers" from "can write agents" within a workspace via Keycloak
  client roles attached to the group.
- **F2. List members of a workspace's group** via the Keycloak Admin
  API.
- **F3. Many-to-many workspace↔group** binding.
- **F4. Keycloak reconciler ("operator")** — automate group creation
  / rename / deletion when a workspace's `group_name` changes. Two
  flavors:
  - **In-process**: server calls Keycloak Admin API on workspace
    CRUD. Simple but synchronous; needs failure-mode handling.
  - **Separate operator**: process watching workspace lifecycle via a
    queue / outbox, reconciling Keycloak independently.
  Pull-forward triggers: workspace count ≳ 50, or self-service
  workspace creation requested.
- **F5. SCIM provisioning** — sync external IdP groups into Keycloak.

These items SHOULD be linked from `PLAN.md` once that file is updated
for the next phase.

## Implementation sketch

1. **Migration** adding `workspaces.group_name` and the partial index.
2. **Config keys** `AUTH_GROUPS_CLAIM` (default `groups`) wired into
   env / YAML / default per CLAUDE.md, documented in
   `deploy/.env.example`.
3. **`KeycloakClaims.Groups` + `HasGroup`** in
   `server/internal/auth/claims.go`. Tests for missing claim, empty
   array, match, no-match, and a non-default claim name.
4. **`RequireWorkspaceWrite(extractWorkspaceID)` middleware**. Tests
   for the admin / group-match / no-match / null-group-name matrix.
5. **Refactor MCP server / agent / version write handlers** to use
   `RequireWorkspaceWrite` instead of `RequireAdmin`. Workspace and
   publisher CRUD continue to use `RequireAdmin`.
6. **OpenAPI** — add `groupName` to the `WorkspaceAdmin` schema
   (do not expose on the public `Workspace` schema).
7. **Boot-time warning** — log WARN once if any workspace has
   `group_name IS NOT NULL` but no validated token has carried a
   non-empty `groups` claim within the first ~100 requests.
8. **Documentation** — `deploy/README.md` (or equivalent) gets a
   "Setting up workspace access" section walking through the manual
   Keycloak steps.
9. **Tests**:
   - `KeycloakClaims.Groups` with the configurable claim name.
   - Middleware matrix.
   - Integration test: workspace with a group, write endpoint hit
     with matching / non-matching / admin tokens (200 / 403 / 200).
   - Public reads ignore `group_name` entirely.
   - Redirect test asserting that legacy paths 301 *and* the auth
     check is enforced on the redirect target, not the source.

## References

- [ADR 0001 — Workspaces under publishers](0001-workspaces-under-publishers.md).
- [CLAUDE.md](../../CLAUDE.md) — configuration rule, no-user-table,
  OIDC.
- [server/internal/auth/middleware.go](../../server/internal/auth/middleware.go) — current `RequireAdmin`.
- Keycloak group-membership mapper:
  <https://www.keycloak.org/docs/latest/server_admin/#_protocol-mappers>
